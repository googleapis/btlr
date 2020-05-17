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
	"os"
	"os/signal"
	"syscall"
)

const (
	FailedCmdExitCode = 2
	MisuseExitCode    = 50
	InterruptExitCode = 51
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

// contextWithSignalCancel schedules a context to be canceled when it receives sigint/sigterm signals.
func contextWithSignalCancel(ctx context.Context) context.Context {
	ctx, cancel := context.WithCancel(ctx)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		_ = <-sigs
		cancel()
	}()
	return ctx
}
