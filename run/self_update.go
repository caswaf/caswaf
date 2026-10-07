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
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/beego/beego"
	"github.com/casbin/caswaf/util"
)

var (
	selfRepoPath   string
	selfStartHash  string
	selfFailedHash string
	selfBinaryPath string
)

func isSelfManagedHost() bool {
	return runtime.GOOS == "windows" && strings.Count(util.GetHostname(), "-") != 2
}

// InitSelfUpdate remembers the commit this process was started from, as "go run" compiles the code checked out at
// start. It returns false when self update does not apply, e.g. on a development machine.
func InitSelfUpdate() bool {
	if !isSelfManagedHost() {
		return false
	}

	path, err := os.Getwd()
	if err != nil || !util.FileExist(filepath.Join(path, ".git")) {
		return false
	}

	hash, err := gitGetLatestCommitHash(path)
	if err != nil {
		fmt.Printf("InitSelfUpdate() error: %v\n", err)
		return false
	}

	selfRepoPath = path
	selfStartHash = hash
	fmt.Printf("InitSelfUpdate(): [%s] running %s\n", path, hash)
	return true
}

// PrepareSelfUpdate pulls the CasWAF code and builds it into a binary to restart on, which also warms the build cache
// for the "go run" of the Startup shortcut at boot. It returns true when a newer commit than the running one is
// checked out and builds.
func PrepareSelfUpdate() (bool, error) {
	path := selfRepoPath

	_, err := runGitCommand(path, "fetch")
	if err != nil {
		return false, err
	}
	upstream, err := runGitCommand(path, "rev-parse", "@{u}")
	if err != nil {
		return false, err
	}
	upstream = strings.TrimSpace(upstream)
	if upstream == selfFailedHash {
		return false, nil
	}

	oldHash, err := gitGetLatestCommitHash(path)
	if err != nil {
		return false, err
	}
	localDiff, err := gitDiffHead(path)
	if err != nil {
		return false, err
	}

	if oldHash != upstream {
		_, err = gitPull(path)
		if err != nil {
			return false, err
		}
	}

	newHash, err := gitGetLatestCommitHash(path)
	if err != nil {
		return false, err
	}
	if newHash == selfStartHash {
		return false, nil
	}

	fmt.Printf("PrepareSelfUpdate(): [%s] building %s, running %s\n", path, newHash, selfStartHash)
	err = buildSelf(path, newHash)
	if err != nil {
		selfFailedHash = upstream
		if newHash != oldHash {
			err2 := gitRestoreCommit(path, oldHash, localDiff)
			if err2 != nil {
				return false, fmt.Errorf("%s, and restoring %s failed: %s", err.Error(), oldHash, err2.Error())
			}
		}
		return false, fmt.Errorf("%s, kept running %s", err.Error(), selfStartHash)
	}

	webChanged, err := runGitCommand(path, "diff", "--name-only", selfStartHash, newHash, "--", "web")
	if err == nil && strings.TrimSpace(webChanged) != "" {
		// the gateway matters more than the admin UI, so a failed UI build does not hold the update back
		err = gitWebBuild(path)
		if err != nil {
			fmt.Printf("PrepareSelfUpdate(): gitWebBuild() error: %v\n", err)
		}
	}

	return true, nil
}

// buildSelf builds a binary kept next to the binaries of the sites, as a virus scanner (e.g. 360 on de-machine) may
// block the temporary exe of "go run" but not a built one
func buildSelf(path string, hash string) error {
	name := beego.AppConfig.String("dbName")
	binaryPath := filepath.Join(beego.AppConfig.String("appDir"), binaryDirName, name, fmt.Sprintf("%s_%s.exe", name, hash[:12]))

	binaryBuildLock.Lock()
	defer binaryBuildLock.Unlock()

	err := buildBinary(path, binaryPath)
	if err != nil {
		return err
	}

	selfBinaryPath = binaryPath
	return nil
}

// RestartSelf starts CasWAF again from the binary built by PrepareSelfUpdate a few seconds later, after this process
// has exited and released the ports, or from its Startup shortcut when there is no binary. The shortcut and the bat
// are left as they are. The caller exits right after.
func RestartSelf() error {
	if selfBinaryPath != "" && util.FileExist(selfBinaryPath) {
		fmt.Printf("RestartSelf(): restarting from %s\n", selfBinaryPath)
		return startBinaryDetachedAfterDelay(beego.AppConfig.String("dbName"), selfRepoPath, selfBinaryPath)
	}

	shortcutPath := getShortcutPath(beego.AppConfig.String("dbName"))
	if !util.FileExist(shortcutPath) {
		return fmt.Errorf("RestartSelf() error, the shortcut to start CasWAF again is not found: %s", shortcutPath)
	}

	fmt.Printf("RestartSelf(): restarting from %s\n", shortcutPath)
	return startDetachedAfterDelay(shortcutPath)
}
