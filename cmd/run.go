// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/google/shlex"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

var runCmd = &cobra.Command{
	Use:   "run \"PATTERN\" -- COMMAND",
	Short: "Run a command into directories containing files that match the specified pattern.",
	Long: strings.TrimSpace(`
Runs a specific command in parallel, targeting multiple directories concurrently.

btlr run \"PATTERN\" -- COMMAND

"PATTERN" is a glob-style pattern that is matched against files against that 
supports bash-style expansion (including globstar "**"). Any folders containing
a file that matches the specified pattern will have the command executed with a
working directory of that folder. Output from each command and a summary of all
commands run will be printed once execution completes`),
	Args: cobra.MinimumNArgs(2),
	RunE: runRun,
}

var (
	gitDiffArgs    string
	interactive    bool
	maxConcurrency int
	maxCmdDur      time.Duration
)

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&gitDiffArgs, "git-diff", "",
		"Limits the directories targeted by run to only be included if changes are detected via \"git diff VAL\".")
	runCmd.Flags().BoolVar(&interactive, "interactive", terminal.IsTerminal(int(os.Stdout.Fd())),
		"Explicitly set to run interactively. If not specified, will attempt to determine automatically if enviroment is a terminal.")
	runCmd.Flags().IntVar(&maxConcurrency, "max-concurrency", runtime.NumCPU(),
		"Limits the number of directories run max-concurrency. Defaults to 3 time the physical number of cores.")
	runCmd.Flags().DurationVar(&maxCmdDur, "max-cmd-duration", 0,
		"Limits the number of time each cmd is allowed to execute for. At the duration, cmds will be sent a SIGINT signal.")
}

func runRun(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pattern := args[0]
	execCmd, err := shlex.Split(strings.Join(args[1:], " "))
	if err != nil {
		return exitWithCode(MisuseExitCode, err)
	}

	cmd.Print("Collecting directories that match pattern...")
	matches, err := rGlob(pattern)
	if err != nil {
		return exitWithCode(MisuseExitCode, err)
	}
	if len(matches) == 0 {
		return exitWithCode(MisuseExitCode, fmt.Errorf("no paths match pattern: '%s'", pattern))
	}
	// From the matching files, reduce to unique directories
	dirs, hist := []string{}, map[string]bool{}
	for _, m := range matches {
		d := filepath.Dir(m)
		if _, seen := hist[d]; !seen {
			dirs = append(dirs, d)
			hist[d] = true
		}
	}
	cmd.Printf("%d collected.\n", len(matches))

	// Check for changed folders with "git diff"
	if gitDiffArgs != "" {
		statusFmt := "Checking for changes with \"git diff\"... [%d of %d complete]."
		cmd.Printf(statusFmt, 0, len(dirs))
		args, err := shlex.Split(gitDiffArgs)
		if err != nil {
			return exitWithCode(MisuseExitCode, err)
		}
		results := startInDirs(ctx, maxConcurrency, append([]string{"git", "diff", "--exit-code"}, args...), dirs)
		// Wait for runs to complete, updating the user periodically
		for range time.Tick(100 * time.Millisecond) {
			ct := 0
			for _, r := range results {
				if r.Done() {
					ct++
				}
			}
			if interactive {
				cmd.Printf("\r"+statusFmt, ct, len(dirs))
			}
			if ct >= len(dirs) {
				break
			}
		}
		cmd.Println()
		// reduce to only directories with changes
		dirs = make([]string, 0, len(dirs))
		for _, r := range results {
			// git diff returns a non-zero exit code if changes are found
			if r.Status != Success {
				dirs = append(dirs, r.Dir)
			}
		}
	}

	statusFmt := "Running command(s)... [%d of %d complete]."
	cmd.Printf(statusFmt, 0, len(dirs))
	results := startInDirs(ctx, maxConcurrency, execCmd, dirs)

	// Wait for runs to complete, outputing the results as they finish
	updateTick := time.NewTicker(100 * time.Millisecond)
	for i := range results {
		cmd.Printf("\n"+"#\n"+"# %s\n"+"#\n"+"\n", results[i].Dir)

		// Wait for the result to finish, or update the user on the status while waiting
		for {
			select {
			case <-updateTick.C:
				if interactive {
					cmd.Printf("\r"+statusFmt, i, len(dirs))
				}
				continue
			case <-results[i].done:
			}
			break
		}
		r := results[i]
		if r.Status == Skipped {
			continue
		}
		cmd.Println(r.Stdall.String())
		if r.Err != nil {
			cmd.Printf("\nerr: %v\n", r.Err)
		}
		cmd.Println()
	}

	// Summarize runs in one place for users
	cmd.Printf("\n" + "#\n" + "# Summary \n" + "#\n" + "\n")
	ct := map[StatusType]int{}
	for _, r := range results {
		ct[r.Status]++
	}
	for _, s := range []StatusType{Success, Failure, Skipped, Error} {
		cmd.Printf("%s: %d, ", s, ct[s])
	}
	cmd.Println("\b\b")
	// For each test, print 80 char wide line in fmt: "path/to/dir....[ STATUS]"
	for _, r := range results {
		if r.Status == Skipped {
			continue
		}
		d := r.Dir
		if len(d) > 67 { // Truncate the directory if it's too wide
			d = d[:67]
		}
		cmd.Printf("%s%s[%8v]\n", d, strings.Repeat(".", 70-len(d)), r.Status)
	}

	if ct[Failure] > 0 || ct[Error] > 0 {
		// this non-zero exitcode is expected, so don't show usage
		cmd.SilenceErrors, cmd.SilenceUsage = true, true
		return exitWithCode(FailedCmdExitCode, nil)
	}

	return nil // Completed successfully!
}

// startInDirs starts a command running in multiple directories.
func startInDirs(ctx context.Context, maxThreads int, execCmd []string, dirs []string) []runResult {
	results, q := make([]runResult, len(dirs)), make(chan *runResult, len(dirs))
	defer close(q)
	// Spin up workers to run the commands in each directory
	for i := 0; i < maxThreads; i++ {
		go func() {
			for r := range q {
				runInDir(ctx, execCmd, r)
			}
		}()
	}
	// Queue up each directory to be run
	for i, d := range dirs {
		results[i].Dir = d
		results[i].done = make(chan bool)
		q <- &results[i]
	}
	return results
}

// runInDir executes the specified commands, reporting results to the provided runResult.
func runInDir(ctx context.Context, execCmd []string, r *runResult) {
	defer close(r.done)
	// set a timeout if neccesary
	if maxCmdDur != 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, maxCmdDur)
		defer cancel()
	}
	// Run the main cmd
	cmd := exec.CommandContext(ctx, execCmd[0], execCmd[1:]...)
	cmd.Dir = r.Dir
	cmd.Stdout, cmd.Stderr = io.MultiWriter(&r.Stdout, &r.Stdall), io.MultiWriter(&r.Stderr, &r.Stdall)
	r.Err = cmd.Run()
	if _, ok := r.Err.(*exec.ExitError); r.Err != nil && !ok {
		r.Status = Error // If it's not an exit error, the command failed to run
		// A canceled context means that a sigint or sigterm was received
		if r.Err == context.Canceled {
			r.Err = errors.New("interupted before complete (sigint or sigterm)")
		}
		r.Err = fmt.Errorf("failed to run cmd (%s): %w", strings.Join(cmd.Args, " "), r.Err)
		return
	}
	if cmd.ProcessState.Success() {
		r.Status = Success
	} else {
		r.Status = Failure
	}
}

// runResult represents a running command in a specific directory.
type runResult struct {
	Dir    string
	Stdout bytes.Buffer
	Stderr bytes.Buffer
	Stdall bytes.Buffer
	Status StatusType
	Err    error     // err return by cmd
	done   chan bool // closed once the cmd is completed
}

// Done returns if the command is no longer running.
func (r *runResult) Done() bool {
	select {
	case <-r.done:
		return true
	default:
	}
	return false
}

type StatusType string

const (
	Error   StatusType = "ERROR"
	Skipped StatusType = "SKIPPED"
	Failure StatusType = "FAILURE"
	Success StatusType = "SUCCESS"
)

// rGlob returns a slice of filepaths matching a pattern just like `filepath.Glob`, with additional support for globstars (**).
func rGlob(pattern string) (matches []string, err error) {
	parts := strings.Split(pattern, string(os.PathSeparator))
	// Find the index of the first globstar pattern (if any)
	g := -1
	for i := range parts {
		if parts[i] == "**" {
			g = i
			break
		}
	}
	if g == -1 { // If no globstars, use regular glob
		return filepath.Glob(pattern)
	}
	pre, post := filepath.Clean(filepath.Join(parts[:g]...)), filepath.Join(parts[g+1:]...)
	if filepath.IsAbs(pattern) && !filepath.IsAbs(pre) {
		pre = filepath.Join(string(os.PathSeparator), pre)
	}
	if g == len(parts)-1 { // If the globstar is at the end, match all files
		post = "*"
	}
	// Traverse the directory lexicographically, and collect all matching files
	hist := map[string]bool{}
	err = filepath.Walk(pre, func(path string, info os.FileInfo, err error) error {
		if err != nil { // filepath.Glob ignores access errors, so we will too
			return nil
		}
		var results []string
		if info.IsDir() { // Recurse deeper for for directories
			results, err = rGlob(filepath.Join(path, post))
			if err != nil {
				return err
			}
			for _, f := range results {
				if _, seen := hist[f]; !seen {
					hist[f] = true
					matches = append(matches, f)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}
