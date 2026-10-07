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
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// startBat runs the bat in a new console window, with the same command line as when its shortcut is opened
// (cmd.exe /c ""<bat>" "), so that getPid() and getSiteCmds() find it, and the window title of the shortcut
func startBat(title string, batPath string) error {
	comSpec := os.Getenv("ComSpec")
	if comSpec == "" {
		comSpec = `C:\Windows\System32\cmd.exe`
	}

	cmd := exec.Command(comSpec)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: fmt.Sprintf(`cmd.exe /C start "%s" %s /c ""%s" "`, title, comSpec, filepath.FromSlash(batPath)),
	}
	return cmd.Run()
}
