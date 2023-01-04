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
	"errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRun(t *testing.T) {
	// Create temp directory with content
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Failure setting up tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	files := []string{
		filepath.Join(dir, "foo", "foo.txt"),
		filepath.Join(dir, "foo", "bar.txt"),
		filepath.Join(dir, "bar", "bar.txt"),
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f), os.ModePerm); err != nil {
			t.Fatalf("Failure to set up test file dir: %v", err)
		}
		if err := ioutil.WriteFile(f, []byte("hello"), os.ModePerm); err != nil {
			t.Fatalf("Failure to set up test file: %v", err)
		}
	}

	var rmCmd string
	switch o := runtime.GOOS; o {
	case "windows":
		rmCmd = "del"
	default: // linux, darwin
		rmCmd = "rm"
	}

	output, _ := ExecCmd(NewCommand(), "run", filepath.Join(dir, "**", "*.txt"), rmCmd, "foo.txt")
	outcomes := []struct {
		contains string
		want     bool
	}{
		{"[ FAILURE]", true},
		{"[ SUCCESS]", true},
	}
	for _, o := range outcomes {
		if strings.Contains(output, o.contains) != o.want {
			if o.want {
				t.Errorf("want: contains %q, got: \n %s", o.contains, output)
			} else {
				t.Errorf("want: doesn't contain %q, got: \n %s", o.contains, output)
			}

		}
	}
}

func TestMultiTest(t *testing.T) {
	// Create temp directory with content
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Failure setting up tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	files := []string{
		filepath.Join(dir, "foo", "foo.txt"),
		filepath.Join(dir, "foo", "bar.txt"),
		filepath.Join(dir, "bar", "bar.txt"),
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f), os.ModePerm); err != nil {
			t.Fatalf("Failure to set up test file dir: %v", err)
		}
		if err := ioutil.WriteFile(f, []byte("hello"), os.ModePerm); err != nil {
			t.Fatalf("Failure to set up test file: %v", err)
		}
	}

	var rmCmd string
	switch o := runtime.GOOS; o {
	case "windows":
		rmCmd = "del"
	default: // linux, darwin
		rmCmd = "rm"
	}

	output, _ := ExecCmd(NewCommand(), "run", filepath.Join(dir, "foo", ""), filepath.Join(dir, "bar", ""), "--", rmCmd, "foo.txt")
	outcomes := []struct {
		contains string
		want     bool
	}{
		{"[ FAILURE]", true},
		{"[ SUCCESS]", true},
	}
	for _, o := range outcomes {
		if strings.Contains(output, o.contains) != o.want {
			if o.want {
				t.Errorf("want: contains %q, got: \n %s", o.contains, output)
			} else {
				t.Errorf("want: doesn't contain %q, got: \n %s", o.contains, output)
			}

		}
	}
}

func TestGitDiff(t *testing.T) {
	// Create temp directory with content
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Failure setting up tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	files := []string{
		filepath.Join(dir, "foo", "foo.txt"),
		filepath.Join(dir, "foo", "bar.txt"),
		filepath.Join(dir, "bar", "bar.txt"),
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f), os.ModePerm); err != nil {
			t.Fatalf("Failure to set up test file dir: %v", err)
		}
		if err := ioutil.WriteFile(f, []byte("hello"), os.ModePerm); err != nil {
			t.Fatalf("Failure to set up test file: %v", err)
		}
	}

	// Create some git changes to diff against
	args := [][]string{
		{"init", "--initial-branch=main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "tests"},
		{"add", "foo"},
		{"commit", "-m", "test commit"},
		{"add", "bar"},
	}
	for _, a := range args {
		c := exec.Command("git", a...)
		c.Dir = dir
		var buf bytes.Buffer
		c.Stdout, c.Stderr = &buf, &buf
		if err := c.Run(); err != nil {
			t.Log(buf.String())
			t.Fatalf("Failed to set up git in test dir: %v", err)
		}
		t.Log(buf.String())
	}

	var rmCmd string
	switch o := runtime.GOOS; o {
	case "windows":
		rmCmd = "del"
	default: // linux, darwin
		rmCmd = "rm"
	}

	output, err := ExecCmd(NewCommand(), "run", "--git-diff=main .", filepath.Join(dir, "**", "*.txt"), rmCmd, "bar.txt")
	if err != nil {
		t.Errorf("btlr run failed: %v", err)
	}

	outcomes := []struct {
		contains string
		want     bool
	}{
		{filepath.Dir(files[1]), false},
		{filepath.Dir(files[2]), true},
	}
	for _, o := range outcomes {
		if strings.Contains(output, o.contains) != o.want {
			if o.want {
				t.Errorf("want: contains %q, got: \n %s", o.contains, output)
			} else {
				t.Errorf("want: doesn't contain %q, got: \n %s", o.contains, output)
			}
		}
	}
}

func TestMaxCmdDur(t *testing.T) {
	// Create temp directory with content
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Failure setting up tempdir: %v", err)
	}
	defer os.RemoveAll(dir)
	files := []string{
		filepath.Join(dir, "foo", "foo.txt"),
		filepath.Join(dir, "bar", "bar.txt"),
	}
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f), os.ModePerm); err != nil {
			t.Fatalf("Failure to set up test file dir: %v", err)
		}
		if err := ioutil.WriteFile(f, []byte("hello"), os.ModePerm); err != nil {
			t.Fatalf("Failure to set up test file: %v", err)
		}
	}

	var sleepCmd string
	switch o := runtime.GOOS; o {
	case "windows":
		sleepCmd = "timeout 2"
	default: // linux, darwin
		sleepCmd = "sleep 2"
	}

	output, err := ExecCmd(NewCommand(), "run", "--max-cmd-duration=1s", filepath.Join(dir, "**", "*.txt"), sleepCmd)
	if err != nil {
		var eErr *exitError
		if !errors.As(err, &eErr) || eErr.Code != 2 {
			t.Fatalf("btlr run failed: %v", err)
		}
	}

	w := "signal: killed"
	if !strings.Contains(output, w) {
		t.Errorf("want %q, got: \n %s", w, output)
	}
}

func TestRGlob(t *testing.T) {
	// Create temp directory with content
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Failure setting up tempdir: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failure to get cwd: %v", err)
	}
	defer func() { // clean up
		_ = os.Chdir(cwd)
		_ = os.RemoveAll(dir)
	}()
	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("Failure to move into tempdir: %v", err)
	}
	content := []string{
		"file.txt",
		"file.xml",
		filepath.Join("a", "file.txt"),
		filepath.Join("a", "b", "c", "file.txt"),
		filepath.Join("a", "b", "c", "file.xml"),
		filepath.Join("a", "b", "c", "d", "file.txt"),
	}
	for _, f := range content {
		if err := os.MkdirAll(filepath.Dir(f), os.ModePerm); err != nil {
			t.Fatalf("Failure to set up test file dir: %v", err)
		}
		if err := ioutil.WriteFile(f, []byte("hello"), os.ModePerm); err != nil {
			t.Fatalf("Failure to set up test file: %v", err)
		}
	}

	cases := []struct {
		desc    string
		pattern string
		want    []string
	}{
		{
			"basic glob",
			"*.txt",
			[]string{
				"file.txt",
			},
		},
		{
			"basic globstar",
			"**.txt",
			[]string{
				"file.txt",
			},
		},
		{
			"folder globstar",
			filepath.Join("**", "*.txt"),
			[]string{
				"file.txt",
				filepath.Join("a", "file.txt"),
				filepath.Join("a", "b", "c", "file.txt"),
				filepath.Join("a", "b", "c", "d", "file.txt"),
			},
		},
		{
			"double globstar",
			filepath.Join("**", "b", "**", "*.txt"),
			[]string{
				filepath.Join("a", "b", "c", "file.txt"),
				filepath.Join("a", "b", "c", "d", "file.txt"),
			},
		},
	}

	for _, c := range cases {
		got, err := rGlob(c.pattern)
		if err != nil {
			t.Errorf("%s: pattern '%s' returned error from rGlob: %v", c.desc, c.pattern, err)
			continue
		}
		if ok := equalStr(c.want, got); !ok {
			t.Errorf("%s: wrong match for pattern '%s' (got: %v, want: %v)", c.desc, c.pattern, got, c.want)
		}
	}
}

// ExecCmd runs a cobra command and return the output.
func ExecCmd(cmd *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()

	return buf.String(), err
}

// equalStr returns true if slices contain the equal elements.
func equalStr(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}
