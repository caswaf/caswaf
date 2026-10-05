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
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// a restart that starts a new instance takes a while to build and take over the port,
// so a site is restarted again only if it is still running its old code after this interval
const restartRetryInterval = 10 * time.Minute

var (
	restartTimeMap  = map[string]time.Time{}
	restartTimeLock = &sync.Mutex{}
)

func isHandoverRepo(siteName string) bool {
	// the new instance of these repos kills the old one on its port by itself (util.StopOldInstance() in their main.go),
	// so the old one keeps serving while the new one is being built
	for _, prefix := range []string{"casdoor", "casibase", "opendata", "openct", "casos"} {
		if strings.HasPrefix(siteName, prefix) {
			return true
		}
	}
	return false
}

func restartProcess(siteName string, useBinary bool) (int, error) {
	restartTimeLock.Lock()
	restartTimeMap[siteName] = time.Now()
	restartTimeLock.Unlock()

	wantBinary := shouldRunBinary(siteName, useBinary)
	needSwitchBat := isBinaryBat(siteName) != wantBinary
	if !isHandoverRepo(siteName) || needSwitchBat {
		err := stopProcess(siteName)
		if err != nil {
			return 0, fmt.Errorf("stopProcess(): %s", err.Error())
		}
	}

	if needSwitchBat {
		err := switchBatFile(siteName, wantBinary)
		if err != nil {
			return 0, fmt.Errorf("switchBatFile(): %s", err.Error())
		}
	}

	err := startProcess(siteName)
	if err != nil {
		return 0, fmt.Errorf("startProcess(): %s", err.Error())
	}

	pid, err := getPid(siteName)
	if err != nil {
		return 0, fmt.Errorf("getPid(): %s", err.Error())
	}

	return pid, nil
}

// getListenerInfo returns the start time and executable of the process listening on the port and the command line
// of the cmd.exe it runs in (e.g. the one of "Desktop\run\casdoor.bat"), or a zero time if nothing listens on it
func getListenerInfo(port int) (time.Time, string, string, error) {
	psCommand := fmt.Sprintf(`$c = Get-NetTCPConnection -LocalPort %d -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
if ($c) {
	$p = Get-CimInstance Win32_Process -Filter "ProcessId=$($c.OwningProcess)"
	$cmdLine = ""
	$q = $p
	for ($i = 0; $i -lt 3 -and $q; $i++) {
		$q = Get-CimInstance Win32_Process -Filter "ProcessId=$($q.ParentProcessId)"
		if ($q -and $q.Name -eq "cmd.exe") { $cmdLine = $q.CommandLine; break }
	}
	"$(([DateTimeOffset]$p.CreationDate).ToUnixTimeSeconds())|$($p.ExecutablePath)|$cmdLine"
}
exit 0`, port)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCommand)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return time.Time{}, "", "", fmt.Errorf("powershell command failed: %v, stderr: %s", err, stderr.String())
	}

	output := strings.TrimSpace(out.String())
	if output == "" {
		return time.Time{}, "", "", nil
	}

	tokens := strings.SplitN(output, "|", 3)
	if len(tokens) != 3 {
		return time.Time{}, "", "", fmt.Errorf("getListenerInfo() error, unexpected output: %s", output)
	}
	startTime, err := strconv.ParseInt(tokens[0], 10, 64)
	if err != nil {
		return time.Time{}, "", "", err
	}

	return time.Unix(startTime, 0), tokens[1], tokens[2], nil
}

// getSiteCmdPids returns the pids of the cmd.exe running this site's bat, i.e. the running instance
// and any new instance that is still being built
func getSiteCmdPids(siteName string) ([]int, error) {
	psCommand := `Get-CimInstance Win32_Process -Filter "Name='cmd.exe'" | ForEach-Object { "$($_.CommandLine) $($_.ProcessId)" }`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCommand)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("powershell command failed: %v, stderr: %s", err, stderr.String())
	}

	res := []int{}
	name := getMappedName(siteName)
	for _, line := range strings.Split(strings.ReplaceAll(out.String(), "\r", ""), "\n") {
		if parseBatName(line) != name {
			continue
		}
		tokens := strings.Fields(line)
		pid, err := strconv.Atoi(tokens[len(tokens)-1])
		if err == nil {
			res = append(res, pid)
		}
	}
	return res, nil
}

// gitGetHeadCheckoutTime returns when HEAD became the current commit (e.g. by a pull), from the reflog.
// The later entries of the same commit, which "git pull --autostash" and "git reset" also write, are skipped.
func gitGetHeadCheckoutTime(path string) (time.Time, error) {
	out, err := runGitCommand(path, "reflog", "-n", "1000", "--date=unix", "--format=%H %gd")
	if err != nil {
		return time.Time{}, err
	}

	res := ""
	head := ""
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		tokens := strings.SplitN(strings.TrimSpace(line), " ", 2)
		if len(tokens) != 2 {
			continue
		}

		if head == "" {
			head = tokens[0]
		} else if tokens[0] != head {
			break
		}
		// e.g. "HEAD@{1727600000}"
		res = tokens[1]
	}

	start := strings.Index(res, "{")
	end := strings.Index(res, "}")
	if start == -1 || end < start {
		return time.Time{}, fmt.Errorf("gitGetHeadCheckoutTime() error, unexpected reflog: %s", out)
	}
	checkoutTime, err := strconv.ParseInt(res[start+1:end], 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(checkoutTime, 0), nil
}

// RestartIfStale restarts the site if the process serving its port started before the current code was checked out,
// e.g. when the new instance started after a pull exited before taking over the port. It returns the pid of the new
// instance (0 if not restarted) and a message saying the site runs old code (empty if not).
func RestartIfStale(siteName string, port int, useBinary bool) (int, string, error) {
	if runtime.GOOS != "windows" || port == 0 {
		return 0, "", nil
	}

	err := checkSharedCode(siteName)
	if err != nil {
		return 0, "", err
	}

	startTime, exePath, cmdLine, err := getListenerInfo(port)
	if err != nil {
		return 0, "", err
	}
	// only judge a process started from this site's own bat, not e.g. a docker proxy on the same port
	if startTime.IsZero() || parseBatName(cmdLine) != getMappedName(siteName) {
		return 0, "", nil
	}

	msg := ""
	if shouldRunBinary(siteName, useBinary) {
		binaryPath := getRunningBinaryPath(siteName)
		if binaryPath == "" || (strings.EqualFold(exePath, binaryPath) && isBinaryBat(siteName)) {
			return 0, "", nil
		}

		msg = fmt.Sprintf("the process on port %d runs %s instead of %s", port, exePath, binaryPath)
	} else if isBinaryBat(siteName) {
		msg = fmt.Sprintf("the process on port %d runs %s instead of \"go run\"", port, exePath)
	} else {
		checkoutTime, err := gitGetHeadCheckoutTime(GetRepoPath(siteName))
		if err != nil {
			return 0, "", err
		}
		if !startTime.Before(checkoutTime) {
			return 0, "", nil
		}

		msg = fmt.Sprintf("the process on port %d started at %s runs older code than the one checked out at %s",
			port, startTime.Format(time.RFC3339), checkoutTime.Format(time.RFC3339))
	}

	restartTimeLock.Lock()
	lastRestartTime, ok := restartTimeMap[siteName]
	restartTimeLock.Unlock()
	if ok && time.Since(lastRestartTime) < restartRetryInterval {
		return 0, fmt.Sprintf("%s, waiting for the restart at %s", msg, lastRestartTime.Format(time.RFC3339)), nil
	}

	// a new instance started before may still be building (e.g. on a busy machine), starting one more
	// would only pile up builds, so wait until it takes over the port or exits
	cmdPids, err := getSiteCmdPids(siteName)
	if err != nil {
		return 0, msg, err
	}
	if len(cmdPids) > 1 {
		return 0, fmt.Sprintf("%s, waiting for the new instance to take over, the running cmd.exe of the site: %v", msg, cmdPids), nil
	}

	fmt.Printf("RestartIfStale(): [%s] %s, restarting\n", siteName, msg)
	pid, err := restartProcess(siteName, useBinary)
	if err != nil {
		return 0, msg, err
	}
	return pid, fmt.Sprintf("%s, restarted at %s", msg, time.Now().Format(time.RFC3339)), nil
}
