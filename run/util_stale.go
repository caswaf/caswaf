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
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// a restart that starts a new instance takes a while to build and take over the port,
// so a site is restarted again only if it is still running its old code after this interval, unless the code to run
// changed since the last restart (e.g. two commits pushed minutes apart)
const restartRetryInterval = 10 * time.Minute

var (
	restartTimeMap   = map[string]time.Time{}
	restartTargetMap = map[string]string{}
	restartTimeLock  = &sync.Mutex{}
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
	restartTargetMap[siteName] = ""
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

// getListeningPid returns the pid of the process listening on the TCP port, or 0 if nothing listens on it. It reads
// "netstat -ano" instead of Get-NetTCPConnection, which lists every connection of the machine first and takes ~10
// seconds on a gateway with thousands of them, once for every site in every round
func getListeningPid(port int) (int, error) {
	out, err := exec.Command("netstat", "-ano").Output()
	if err != nil {
		return 0, fmt.Errorf("netstat failed: %v", err)
	}

	suffix := fmt.Sprintf(":%d", port)
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		// TCP  0.0.0.0:8000  0.0.0.0:0  LISTENING  1234, the state column may be localized but a listener has no
		// remote port
		if len(fields) != 5 || !strings.EqualFold(fields[0], "TCP") || !strings.HasSuffix(fields[1], suffix) || !strings.HasSuffix(fields[2], ":0") {
			continue
		}
		pid, err := strconv.Atoi(fields[4])
		if err == nil && pid > 0 {
			return pid, nil
		}
	}
	return 0, nil
}

// getListenerInfo returns the start time and executable of the process listening on the port and the command line
// of the cmd.exe it runs in (e.g. the one of "Desktop\run\casdoor.bat"), or a zero time if nothing listens on it
func getListenerInfo(port int) (time.Time, string, string, error) {
	pid, err := getListeningPid(port)
	if err != nil || pid == 0 {
		return time.Time{}, "", "", err
	}

	p, cmd, err := getProcessAndCmd(pid)
	if err != nil || p == nil {
		return time.Time{}, "", "", err
	}
	cmdLine := ""
	if cmd != nil {
		cmdLine = cmd.cmdLine
	}
	return p.startTime, p.exePath, cmdLine, nil
}

type siteCmd struct {
	pid       int
	startTime time.Time
}

// getSiteCmds returns the cmd.exe running this site's bat, i.e. the running instance
// and any new instance that is still being built
func getSiteCmds(siteName string) ([]siteCmd, error) {
	if !isWindows() {
		pid, err := getUnitPid(siteName)
		if err != nil || pid == 0 {
			return []siteCmd{}, err
		}
		return []siteCmd{{pid: pid}}, nil
	}

	processes, err := getCmdProcesses()
	if err != nil {
		return nil, err
	}

	res := []siteCmd{}
	name := getMappedName(siteName)
	for _, p := range processes {
		if parseBatName(p.cmdLine) == name {
			res = append(res, siteCmd{pid: p.pid, startTime: p.startTime})
		}
	}
	return res, nil
}

func getSiteCmdPids(siteName string) ([]int, error) {
	cmds, err := getSiteCmds(siteName)
	if err != nil {
		return nil, err
	}

	res := []int{}
	for _, cmd := range cmds {
		res = append(res, cmd.pid)
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
	target := ""
	if shouldRunBinary(siteName, useBinary) {
		binaryPath := getRunningBinaryPath(siteName)
		if binaryPath == "" || (strings.EqualFold(exePath, binaryPath) && isBinaryBat(siteName)) {
			return 0, "", nil
		}

		msg = fmt.Sprintf("the process on port %d runs %s instead of %s", port, exePath, binaryPath)
		target = binaryPath
	} else if isBinaryBat(siteName) {
		msg = fmt.Sprintf("the process on port %d runs %s instead of \"go run\"", port, exePath)
		target = "go run"
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
		target = checkoutTime.Format(time.RFC3339)
	}

	restartTimeLock.Lock()
	lastRestartTime, ok := restartTimeMap[siteName]
	lastTarget := restartTargetMap[siteName]
	restartTimeLock.Unlock()
	if ok && time.Since(lastRestartTime) < restartRetryInterval && (lastTarget == "" || lastTarget == target) {
		return 0, fmt.Sprintf("%s, waiting for the restart at %s", msg, lastRestartTime.Format(time.RFC3339)), nil
	}

	// a new instance started before may still be building (e.g. on a busy machine), starting one more
	// would only pile up builds, so wait until it takes over the port or exits
	cmds, err := getSiteCmds(siteName)
	if err != nil {
		return 0, msg, err
	}
	newerPids := []int{}
	for _, cmd := range cmds {
		if cmd.startTime.After(startTime) {
			newerPids = append(newerPids, cmd.pid)
		}
	}
	if len(newerPids) > 0 {
		return 0, fmt.Sprintf("%s, waiting for the new instance to take over, the newer cmd.exe of the site: %v", msg, newerPids), nil
	}

	fmt.Printf("RestartIfStale(): [%s] %s, restarting\n", siteName, msg)
	pid, err := restartProcess(siteName, useBinary)
	restartTimeLock.Lock()
	restartTargetMap[siteName] = target
	restartTimeLock.Unlock()
	if err != nil {
		return 0, msg, err
	}
	return pid, fmt.Sprintf("%s, restarted at %s", msg, time.Now().Format(time.RFC3339)), nil
}
