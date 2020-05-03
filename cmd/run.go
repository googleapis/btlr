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
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:   "run PATTERN -- SUBCOMMAND",
	Short: "Run a command into directories containing files that match the specified pattern.",
	Long: `Run a command into directories containing files that match the specified pattern.`,
	Args: cobra.MinimumNArgs(2),
	RunE: runRun,
}

func runRun(cmd *cobra.Command, args []string) error {
	pattern := args[0]
	execCmd, execArgs := args[1], args[2:]

	// Find all files matching the pattern
	matches, err := rGlob(pattern)
	if err != nil {
		return exitWithCode(MisuseExitCode, err)
	}
	if matches == nil {
		return exitWithCode(MisuseExitCode, fmt.Errorf("No paths match pattern: '%s'", pattern))
	}

	// From the matching files, narrow down to list of unique directories
	dirs, hist := []string{}, map[string]bool{}
	for _, m := range matches {
		d := filepath.Dir(m)
		if _, seen := hist[d]; !seen {
			dirs = append(dirs, d)
			hist[d] = true
		}
	}

	// Create a context for propagating Interrupt/Terminate signals
	ctx, cancel := context.WithCancel(context.Background())
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		_ = <-sigs
		cancel()
	}()

	// Run the command in each directory
	statusFmt := "Running command... [%d of %d complete]."
	fmt.Printf(statusFmt, 0, len(dirs))
	var runs []*runStatus
	for _, d := range dirs {
		r := startRun(ctx, d, execCmd, execArgs...)
		runs = append(runs, r)
	}

	// Update user with current status periodically
	t := time.Tick(250 * time.Millisecond)
	for {
		// Count completed runs
		ct := 0
		for _, r := range runs {
			if r.Done() {
				ct++
			}
		}
		fmt.Printf("\r"+statusFmt, ct, len(dirs))
		select {
		case <-ctx.Done():
			fmt.Println("\n\nExecution interrupted!")
			os.Exit(InterruptExitCode)
		case <-t:
			if ct < len(runs) {
				continue
			}
		}
		break
	}
	fmt.Println()

	// Report the output of each command
	for _, r := range runs {
		fmt.Printf("\n"+"#\n"+"# %s\n"+"#\n"+"\n", r.Dir)
		fmt.Println(r.Output())
		fmt.Println()
	}

	// Report summary of results
	success := true
	fmt.Printf("\n" + "#\n" + "# Summary \n" + "#\n" + "\n")
	// For each test, print 80 char wide line in fmt: "path/to/dir....[ STATUS]"
	for _, r := range runs {
		// Truncate the dir if it's too wide
		d := r.Dir
		if len(d) > 67 {
			d = d[:67]
		}
		fmt.Print(d)
		fmt.Print(strings.Repeat(".", 70-len(d)))
		if r.Success() {
			fmt.Println("[PASSED]")
		} else {
			fmt.Println("[FAILED]")
			success = false
		}
	}

	if !success {
		os.Exit(10)
	}
	// Completed successfully!
	return nil
}

// startRun starts running a command in a specific directory.
func startRun(ctx context.Context, dir string, name string, arg ...string) *runStatus {
	cmd := exec.CommandContext(ctx, name, arg...)
	cmd.Dir = dir
	done := make(chan bool)
	dc := &runStatus{Dir: dir, cmd: cmd, done: done,}
	go func() {
		dc.output, dc.err = cmd.CombinedOutput()
		close(done)
	}()
	return dc
}

// runStatus represents a running command in a specific directory.
type runStatus struct {
	Dir    string
	cmd    *exec.Cmd
	done   <-chan bool // closed once the cmd is completed
	output []byte      // output returned by cmd
	err    error       // err return by cmd
}

// Done returns if the command is no longer running.
func (rs *runStatus) Done() bool {
	select {
	case <-rs.done:
		return true
	default:
	}
	return false
}

// Success reports whether the command exited successfully, such as with exit status 0 on Unix. Blocks until command is no longer running.
func (rs *runStatus) Success() bool {
	select {
	case <-rs.done:
	}
	return rs.cmd.ProcessState.Success()
}

// Output reports the combined output of the command. Blocks until command is no longer running.
func (rs *runStatus) Output() string {
	select {
	case <-rs.done:
	}
	if rs.err != nil {
		return fmt.Sprintf("Execution failed: %v", rs.err)
	}
	return string(rs.output)
}

// rGlob returns a slice of filepaths matching a pattern just like `filepath.Glob`, with additional support for globstars (**).
func rGlob(pattern string) ([]string, error) {
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
	if g == len(parts)-1 { // If the globstar is at the end, match all files
		post = "*"
	}
	// Traverse the directory lexicographically, and collect all matching files
	matches, hist := []string{}, map[string]bool{}
	err := filepath.Walk(pre, func(path string, info os.FileInfo, err error) error {
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
