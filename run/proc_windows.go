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
	"errors"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// procInfo is a running process, read with the Windows API instead of a PowerShell Get-CimInstance, which starts a
// new powershell.exe (~0.5 seconds on a busy node) for every site in every round
type procInfo struct {
	pid       int
	ppid      int
	name      string
	exePath   string
	cmdLine   string
	startTime time.Time
}

// getProcessMap returns the pid, parent pid and executable name of every running process
func getProcessMap() (map[int]*procInfo, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(snapshot)

	res := map[int]*procInfo{}
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		pid := int(entry.ProcessID)
		res[pid] = &procInfo{
			pid:  pid,
			ppid: int(entry.ParentProcessID),
			name: windows.UTF16ToString(entry.ExeFile[:]),
		}
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, err
	}
	return res, nil
}

// fillProcessDetail reads the executable path, command line and start time of the process
func fillProcessDetail(p *procInfo) error {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(p.pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)

	var creationTime, exitTime, kernelTime, userTime windows.Filetime
	err = windows.GetProcessTimes(handle, &creationTime, &exitTime, &kernelTime, &userTime)
	if err != nil {
		return err
	}
	p.startTime = time.Unix(0, creationTime.Nanoseconds())

	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if windows.QueryFullProcessImageName(handle, 0, &buf[0], &size) == nil {
		p.exePath = windows.UTF16ToString(buf[:size])
	}

	cmdLine, err := getProcessCommandLine(handle)
	if err != nil {
		return err
	}
	p.cmdLine = cmdLine
	return nil
}

func getProcessCommandLine(handle windows.Handle) (string, error) {
	var size uint32
	err := windows.NtQueryInformationProcess(handle, windows.ProcessCommandLineInformation, nil, 0, &size)
	if size == 0 {
		return "", err
	}

	buf := make([]byte, size)
	err = windows.NtQueryInformationProcess(handle, windows.ProcessCommandLineInformation, unsafe.Pointer(&buf[0]), size, &size)
	if err != nil {
		return "", err
	}
	return (*windows.NTUnicodeString)(unsafe.Pointer(&buf[0])).String(), nil
}

// getCmdProcesses returns the running cmd.exe with their command lines, e.g. the ones running the bats of the sites
func getCmdProcesses() ([]*procInfo, error) {
	processMap, err := getProcessMap()
	if err != nil {
		return nil, err
	}

	res := []*procInfo{}
	for _, p := range processMap {
		if !strings.EqualFold(p.name, "cmd.exe") {
			continue
		}
		// the process may have exited after the snapshot
		if fillProcessDetail(p) == nil {
			res = append(res, p)
		}
	}
	return res, nil
}

// getProcessAndCmd returns the process and the cmd.exe it runs in (up to 3 levels up), or nil if not found
func getProcessAndCmd(pid int) (*procInfo, *procInfo, error) {
	processMap, err := getProcessMap()
	if err != nil {
		return nil, nil, err
	}

	p, ok := processMap[pid]
	if !ok {
		return nil, nil, nil
	}
	err = fillProcessDetail(p)
	if err != nil {
		return nil, nil, err
	}

	q := p
	for i := 0; i < 3; i++ {
		q, ok = processMap[q.ppid]
		if !ok {
			break
		}
		if strings.EqualFold(q.name, "cmd.exe") {
			// a parent started after the child is another process that reused the pid
			if fillProcessDetail(q) != nil || q.startTime.After(p.startTime) {
				break
			}
			return p, q, nil
		}
	}
	return p, nil, nil
}
