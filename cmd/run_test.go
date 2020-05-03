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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestRGlob(t *testing.T) {
	// Create temp directory with content
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("Failed setting up tempdir: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get cwd: %v", err)
	}
	defer func() { // clean up
		_ = os.Chdir(cwd)
		_ = os.RemoveAll(dir)
	}()
	err = os.Chdir(dir)
	if err != nil {
		t.Fatalf("Failed to move into tempdir: %v", err)
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
			t.Fatalf("Failed to set up test file dir: %v", err)
		}
		if err := ioutil.WriteFile(f, []byte("hello"), os.ModePerm); err != nil {
			t.Fatalf("Failed to set up test file: %v", err)
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
		if ok := EqualStr(c.want, got); !ok {
			t.Errorf("%s: wrong match for pattern '%s' (got: %v, want: %v)", c.desc, c.pattern, got, c.want)
		}
	}
}

// EqualStr returns true if slices contain the equal elements.
func EqualStr(a, b []string) bool {
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
