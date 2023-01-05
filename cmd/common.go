// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package cmd

const (
	FailedCmdExitCode = 2
	MisuseExitCode    = 50
)

// exitError is a typed error to return.
type exitError struct {
	Code int
	Err  error
}

// Error implements error.
func (e *exitError) Error() string {
	if e.Err == nil {
		return "<missing error>"
	}
	return e.Err.Error()
}

// exitWithCode prints exits with the specified error and exit code.
func exitWithCode(code int, err error) *exitError {
	return &exitError{
		Err:  err,
		Code: code,
	}
}
