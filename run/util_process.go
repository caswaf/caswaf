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
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/casbin/caswaf/util"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var reBatNames *regexp.Regexp

func init() {
	reBatNames = regexp.MustCompile(`\\Desktop\\run\\(.*?)\.bat`)
}

func parseBatName(s string) string {
	res := reBatNames.FindStringSubmatch(s)
	if res == nil {
		return ""
	}

	return res[1]
}

func getBatNamesFromOutput(output string) map[string]int {
	batNameMap := map[string]int{}

	output = strings.ReplaceAll(output, "\r", "")
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		tokens := strings.Split(line, " ")
		tokens2 := []string{}
		for _, token := range tokens {
			if token != "" {
				tokens2 = append(tokens2, token)
			}
		}

		if len(tokens2) < 5 || strings.ToLower(tokens2[0]) != `c:\windows\system32\cmd.exe` || tokens2[1] != "/c" {
			continue
		}

		batName := parseBatName(tokens2[2])
		processId := util.ParseInt(tokens2[len(tokens2)-1])
		batNameMap[batName] = processId
		//fmt.Printf("%s, %d\n", batName, processId)
	}

	return batNameMap
}

func getPid(name string) (int, error) {
	name = getMappedName(name)

	psCommand := `Get-CimInstance Win32_Process -Filter "Name='cmd.exe'" | Select-Object CommandLine, ProcessId | ForEach-Object { "$($_.CommandLine) $($_.ProcessId)" }`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCommand)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return 0, fmt.Errorf("powershell command failed: %v, stderr: %s", err, stderr.String())
	}

	batNameMap := getBatNamesFromOutput(out.String())
	pid, ok := batNameMap[name]
	if ok {
		return pid, nil
	} else {
		return 0, fmt.Errorf("getBatNamesFromOutput() error, name = %s, batNameMap = %v", name, batNameMap)
	}
}

func startProcess(name string) error {
	fmt.Printf("startProcess(): [%s]\n", name)

	cmd := exec.Command("cmd", "/C", "start", "", getShortcutPath(name))
	return cmd.Run()
}

func stopProcess(name string) error {
	fmt.Printf("stopProcess(): [%s]\n", name)

	// find the cmd.exe of the site by the bat in its command line, like getPid(), because the window title
	// "casdoor.bat - Shortcut" changes once the program runs another command in the console (e.g. "npm install ...")
	pids, err := getSiteCmdPids(name)
	if err != nil {
		return err
	}
	if len(pids) > 0 {
		for _, pid := range pids {
			err = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
			if err != nil {
				// the process may have exited by itself after it was listed
				active, err2 := IsProcessActive(pid)
				if err2 != nil || active {
					return fmt.Errorf("taskkill /PID %d: %s", pid, err.Error())
				}
			}
		}
		return nil
	}

	// no cmd.exe runs the bat, e.g. the site was started in another way, so fall back to the window title
	name = getMappedName(name)
	windowName := fmt.Sprintf("%s.bat - %s", name, getShortcut())
	// taskkill /IM "casdoor.bat - Shortcut" /F
	// taskkill /F /FI "WINDOWTITLE eq casdoor.bat - Shortcut" /T
	cmd := exec.Command("taskkill", "/F", "/FI", fmt.Sprintf("WINDOWTITLE eq %s", windowName), "/T")
	return cmd.Run()
}

func IsProcessActive(pid int) (bool, error) {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid))
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return false, err
	}

	output := out.String()
	res := strings.Contains(output, strconv.Itoa(pid))
	return res, nil
}

func IsWindowTitleActive(name string) (bool, error) {
	name = getMappedName(name)
	windowName := fmt.Sprintf("%s.bat - %s", name, getShortcut())

	// Use tasklist to check if a window with the specific title exists
	cmd := exec.Command("tasklist", "/V", "/FI", fmt.Sprintf("WINDOWTITLE eq %s", windowName))
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return false, err
	}

	// Decode output from GBK (Windows default codepage) to UTF-8
	decoder := simplifiedchinese.GBK.NewDecoder()
	reader := transform.NewReader(&out, decoder)
	decoded, err := io.ReadAll(reader)
	if err != nil {
		// If decoding fails, fall back to original output
		decoded = out.Bytes()
	}
	output := string(decoded)

	// Check if cmd.exe process with the window title exists
	// If window title is found, output will contain "cmd.exe" and the window title
	res := strings.Contains(output, "cmd.exe") && strings.Contains(output, windowName)
	return res, nil
}
