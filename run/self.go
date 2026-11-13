// Copyright 2023 The casbin Authors. All Rights Reserved.
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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/beego/beego"
	"github.com/casbin/caswaf/util"
)

func InitSelfStart() {
	if runtime.GOOS != "windows" && !isSystemdService() {
		return
	}

	hostname := util.GetHostname()
	if strings.Count(hostname, "-") == 2 {
		return
	}

	siteName := beego.AppConfig.String("dbName")
	if siteName == "" {
		panic("InitSelfStart() error, siteName should not be empty")
	}

	_, err := updateBatFile(siteName)
	if err != nil {
		panic(err)
	}

	// the shortcut only starts CasWAF after a reboot, antivirus software on some nodes blocks creating it,
	// which must not stop CasWAF itself from starting
	shortcutPath := getShortcutPath(siteName)
	if !util.FileExist(shortcutPath) {
		err = updateShortcutFile(siteName)
		if err != nil {
			fmt.Printf("InitSelfStart() error: failed to create the startup shortcut: %s\n", err.Error())
		}
	}

	err = useSelfBinary(siteName)
	if err != nil {
		fmt.Printf("InitSelfStart() error: failed to point the bat to the binary: %s\n", err.Error())
	}
}

// useSelfBinary points the bat (the unit on Linux) of CasWAF to the binary built by self update that this process
// runs from, so that a reboot or a restart from the bat does not "go run" CasWAF again
func useSelfBinary(name string) error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	binaryDir := filepath.Join(beego.AppConfig.String("appDir"), binaryDirName, name)
	if !strings.EqualFold(filepath.Dir(exePath), filepath.Clean(binaryDir)) {
		return nil
	}
	wd, err := os.Getwd()
	if err != nil || !strings.EqualFold(filepath.Clean(wd), filepath.Clean(GetRepoPath(name))) {
		return err
	}

	if !isWindows() {
		return restartSelfUnit(name, exePath)
	}

	err = writeBinaryPointer(name, exePath)
	if err != nil {
		return err
	}
	if isBinaryBat(name) {
		return nil
	}

	// cmd.exe reads a bat line by line, so the bat is not rewritten while a cmd.exe still runs it
	cmds, err := getSiteCmds(name)
	if err != nil {
		return err
	}
	if len(cmds) != 0 {
		fmt.Printf("useSelfBinary(): [%s] the bat is running, kept it\n", name)
		return nil
	}
	return updateBinaryBatFile(name)
}
