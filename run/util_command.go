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

package run

import (
	"os/exec"
	"strconv"
	"time"
)

// commandWaitDelay bounds the wait for the output pipes after a timed out command is killed,
// as its children (git's fetch and ssh, go's compile) may still hold them open
const commandWaitDelay = 30 * time.Second

// killTreeOnCancel makes the timeout of a command kill the processes it started too
func killTreeOnCancel(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if isWindows() {
			_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
		}
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = commandWaitDelay
}
