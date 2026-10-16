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
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/beego/beego"
	"github.com/casbin/caswaf/util"
)

const (
	systemdUnitDir   = "/etc/systemd/system"
	systemdWantsDir  = "/etc/systemd/system/multi-user.target.wants"
	systemdEnvFile   = "/etc/default/caswaf"
	runDirName       = "_run"
	runScriptDirName = ".caswaf"
	siteBuildRetry   = 30 * time.Minute
)

var siteBuildFailureMap = map[string]binaryFailure{}

func isWindows() bool {
	return runtime.GOOS == "windows"
}

func isSystemdService() bool {
	return !isWindows() && os.Getenv("INVOCATION_ID") != ""
}

func getUnitName(name string) string {
	return fmt.Sprintf("caswaf-site-%s.service", getMappedName(name))
}

func getUnitPath(name string) string {
	return filepath.Join(systemdUnitDir, getUnitName(name))
}

func getUnitWantsPath(name string) string {
	return filepath.Join(systemdWantsDir, getUnitName(name))
}

func getRunDir() string {
	return filepath.Join(beego.AppConfig.String("appDir"), runDirName)
}

func getUnitContent(name string, execStart string) string {
	return fmt.Sprintf(`[Unit]
Description=CasWAF site %s
After=network-online.target docker.service
Wants=network-online.target

[Service]
User=%s
WorkingDirectory=%s
EnvironmentFile=-%s
ExecStart=%s
Restart=always
RestartSec=5
TimeoutStopSec=90

[Install]
WantedBy=multi-user.target
`, getMappedName(name), username, GetRepoPath(name), systemdEnvFile, execStart)
}

func getGoRunUnitContent(name string) string {
	if hasRunScript(name) {
		return getUnitContent(name, getRunScriptPath(name, "run"))
	}

	goPath, err := exec.LookPath("go")
	if err != nil {
		goPath = "/usr/bin/env go"
	}
	return getUnitContent(name, fmt.Sprintf("%s run main.go", goPath))
}

func getBinaryUnitContent(name string) string {
	return getUnitContent(name, fmt.Sprintf(`/bin/sh -c 'exec "$(cat %s)"'`, getBinaryPointerPath(name)))
}

func runSystemctl(args ...string) (string, error) {
	out, err := exec.Command("systemctl", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("systemctl %s: %s, output: %s", strings.Join(args, " "), err.Error(), strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func enableUnit(name string) error {
	_, err := runSystemctl("daemon-reload")
	if err != nil {
		return err
	}
	_, err = runSystemctl("enable", getUnitName(name))
	return err
}

func startUnit(name string) error {
	_, err := runSystemctl("daemon-reload")
	if err != nil {
		return err
	}
	_, err = runSystemctl("start", getUnitName(name))
	return err
}

func stopUnit(name string) error {
	if !util.FileExist(getUnitPath(name)) {
		return nil
	}
	_, err := runSystemctl("stop", getUnitName(name))
	return err
}

func getUnitPid(name string) (int, error) {
	if !util.FileExist(getUnitPath(name)) {
		return 0, nil
	}
	out, err := runSystemctl("show", "-p", "MainPID", "--value", getUnitName(name))
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}

func isUnitActive(name string) (bool, error) {
	if !util.FileExist(getUnitPath(name)) {
		return false, nil
	}
	out, _ := exec.Command("systemctl", "is-active", getUnitName(name)).Output()
	state := strings.TrimSpace(string(out))
	return state == "active" || state == "activating" || state == "reloading", nil
}

func isPidActive(pid int) bool {
	return util.FileExist(fmt.Sprintf("/proc/%d", pid))
}

// A repo that is not a Go program (e.g. one deployed with Docker) brings its own scripts: ".caswaf/build" builds the
// checked out commit after a pull, and ".caswaf/run" runs the site in the foreground as the unit's command.
func hasRunScript(name string) bool {
	return !isWindows() && util.FileExist(getRunScriptPath(name, "run"))
}

func getRunScriptPath(name string, script string) string {
	return filepath.Join(GetRepoPath(name), runScriptDirName, script)
}

func getBuiltMarkPath(name string) string {
	return filepath.Join(getRunDir(), getMappedName(name)+".built")
}

func ensureSiteBuilt(name string) (bool, error) {
	path := GetRepoPath(name)
	hash, err := gitGetLatestCommitHash(path)
	if err != nil {
		return false, err
	}

	markPath := getBuiltMarkPath(name)
	if strings.TrimSpace(readFileString(markPath)) == hash {
		return false, nil
	}

	failure, ok := siteBuildFailureMap[name+"/"+hash]
	if ok && time.Since(failure.time) < siteBuildRetry {
		return false, fmt.Errorf("%s (failed at %s)", failure.err.Error(), failure.time.Format(time.RFC3339))
	}

	buildPath := getRunScriptPath(name, "build")
	if util.FileExist(buildPath) {
		fmt.Printf("ensureSiteBuilt(): [%s] building %s\n", name, hash)
		err = runCmd(path, buildPath)
		if err != nil {
			siteBuildFailureMap[name+"/"+hash] = binaryFailure{time: time.Now(), err: fmt.Errorf("%s: %s", buildPath, err.Error())}
			return false, err
		}
	}
	delete(siteBuildFailureMap, name+"/"+hash)

	err = ensureFileFolderExists(filepath.Dir(markPath))
	if err != nil {
		return false, err
	}
	return true, os.WriteFile(markPath, []byte(hash+"\n"), 0644)
}

func updateRunScriptSite(siteName string, path string, needStart bool, useBinary bool) (int, error) {
	built, err := ensureSiteBuilt(siteName)
	if err != nil {
		return wrapRepoError("ensureSiteBuilt", path, err)
	}

	_, err = updateBatFile(siteName)
	if err != nil {
		return wrapRepoError("updateBatFile", path, err)
	}
	ensureShortcutFile(siteName)

	if needStart {
		err = startProcess(siteName)
		if err != nil {
			return wrapRepoError("startProcess", path, err)
		}
		pid, err := getPid(siteName)
		if err != nil {
			return wrapRepoError("getPid", path, err)
		}
		return pid, nil
	}

	if built {
		pid, err := restartProcess(siteName, useBinary)
		if err != nil {
			return wrapRepoError("restartProcess", path, err)
		}
		return pid, nil
	}
	return 0, nil
}

func restartSelfUnit(name string, binaryPath string) error {
	pointerPath := getBinaryPointerPath(name)
	err := ensureFileFolderExists(filepath.Dir(pointerPath))
	if err != nil {
		return err
	}
	err = os.WriteFile(pointerPath, []byte(binaryPath+"\n"), 0644)
	if err != nil {
		return err
	}

	content := getBinaryUnitContent(name)
	if readFileString(getUnitPath(name)) != content {
		err = os.WriteFile(getUnitPath(name), []byte(content), 0644)
		if err != nil {
			return err
		}
	}
	_, err = runSystemctl("daemon-reload")
	return err
}
