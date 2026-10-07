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

//go:build windows

package run

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"
)

const detachedProcess = 0x00000008

// startDetachedAfterDelay opens the shortcut from a cmd.exe that is detached from this process, so that it outlives
// it. The command line is set raw because cmd.exe does not follow the quoting rules Go uses for arguments.
func startDetachedAfterDelay(shortcutPath string) error {
	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       fmt.Sprintf(`cmd.exe /C "ping -n 6 127.0.0.1 >nul & start "" "%s""`, filepath.FromSlash(shortcutPath)),
		CreationFlags: detachedProcess | syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
	return cmd.Start()
}

// startBinaryDetachedAfterDelay runs the binary in a new console window titled with the name and in the repo directory,
// where it reads conf/app.conf, from a cmd.exe that is detached from this process
func startBinaryDetachedAfterDelay(name string, dir string, binaryPath string) error {
	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine:       fmt.Sprintf(`cmd.exe /C "ping -n 6 127.0.0.1 >nul & start "%s" /D "%s" "%s""`, name, filepath.FromSlash(dir), filepath.FromSlash(binaryPath)),
		CreationFlags: detachedProcess | syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
	return cmd.Start()
}
