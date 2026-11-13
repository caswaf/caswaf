// Copyright 2026 The casbin Authors. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build go1.23

package util

import (
	"fmt"
	"os"
	"runtime/debug"
	"time"
)

const maxCrashLogSize = 10 * 1024 * 1024

// initCrashOutput also writes the panic message and stacks of a fatal crash to the file, which the console loses
// when its window is closed
func initCrashOutput(path string) {
	info, err := os.Stat(path)
	if err == nil && info.Size() > maxCrashLogSize {
		os.Rename(path, path+".1")
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Printf("initCrashOutput() error: %v\n", err)
		return
	}

	file.WriteString(fmt.Sprintf("%s started pid = %d, ppid = %d\n", time.Now().Format("2006/01/02 15:04:05.000"), os.Getpid(), os.Getppid()))
	err = debug.SetCrashOutput(file, debug.CrashOptions{})
	if err != nil {
		fmt.Printf("initCrashOutput() error: %v\n", err)
	}
	file.Close()
}
