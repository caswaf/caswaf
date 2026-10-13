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
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/beego/beego"
	"github.com/casbin/caswaf/util"
)

func runCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = dir
	return cmd.Run()
}

func getOriginalName(name string) string {
	tokens := strings.Split(name, "_")
	if len(tokens) > 0 {
		return tokens[0]
	} else {
		return name
	}
}

func getNameIndex(name string) int {
	tokens := strings.Split(name, "_")
	if len(tokens) > 0 {
		return util.ParseInt(tokens[len(tokens)-1])
	} else {
		panic(fmt.Sprintf("getNameIndex() error, name = %s", name))
	}
}

func getRepoUrl(name string) string {
	if name == "casdoor" {
		return "https://github.com/casdoor/casdoor"
	} else if name == "casibase" {
		return "https://github.com/casibase/casibase"
	} else if name == "opendata" {
		return "https://github.com/hsluoyz/opendata"
	} else if name == "openct" {
		return "https://gitee.com/openct/openct"
	} else if name == "casos" {
		return "https://github.com/casosorg/casos"
	} else {
		return fmt.Sprintf("https://github.com/casbin/%s", name)
	}
}

func getShortcut() string {
	res := "Shortcut"
	language := beego.AppConfig.String("language")
	if language != "en" {
		res = "快捷方式"
	}
	return res
}

func ensureFileFolderExists(path string) error {
	if !util.FileExist(path) {
		return os.MkdirAll(path, os.ModePerm)
	}
	return nil
}

var initAdminPasswordRegex = regexp.MustCompile(`(?m)^initAdminPassword\s*=.*$`)

func setInitAdminPassword(content string, password string) string {
	line := fmt.Sprintf("initAdminPassword = \"%s\"", password)
	if initAdminPasswordRegex.MatchString(content) {
		return initAdminPasswordRegex.ReplaceAllLiteralString(content, line)
	}
	return strings.TrimRight(content, "\r\n") + "\n" + line + "\n"
}

// setAppConfValue sets "key = value" in the content of an app.conf file: the first uncommented line
// for the key is replaced whatever its current value is, and the line is appended if there is none.
func setAppConfValue(content string, key string, value string) string {
	newLine := fmt.Sprintf("%s = %s", key, value)

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		tokens := strings.SplitN(line, "=", 2)
		if len(tokens) != 2 || strings.TrimSpace(tokens[0]) != key {
			continue
		}

		if strings.HasSuffix(line, "\r") {
			newLine += "\r"
		}
		lines[i] = newLine
		return strings.Join(lines, "\n")
	}

	eol := "\n"
	if strings.Contains(content, "\r\n") {
		eol = "\r\n"
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += eol
	}
	return content + newLine + eol
}

func updateAppConfFile(name string, i int, orgName string, initAdminPassword string) {
	fmt.Printf("updateAppConfFile(): [%s]\n", name)
	confPath := getCodeAppConfPath(name)
	content := util.ReadStringFromPath(confPath)

	if strings.HasPrefix(name, "casibase_customer_") {
		shortName := strings.ReplaceAll(name, "casibase_customer_", "cbc")
		if orgName != "" {
			shortName = orgName
		}

		// openagent reads the Casdoor endpoint from "issuer" and falls back to "casdoorEndpoint", but still uses
		// "casdoorEndpoint" to pick the CDN and to fill the built-in site, so both are set.
		casdoorEndpoint := strings.ReplaceAll(beego.AppConfig.String("casdoorEndpoint"), "my.", "cbc.")
		content = setAppConfValue(content, "httpport", fmt.Sprintf("%d", 40000+i))
		content = setAppConfValue(content, "dataSourceName", fmt.Sprintf("%s:%s@tcp(%s:3306)/", beego.AppConfig.String("dbUser"), beego.AppConfig.String("dbPass"), beego.AppConfig.String("dbHost")))
		content = setAppConfValue(content, "dbName", name)
		content = setAppConfValue(content, "redisEndpoint", fmt.Sprintf("\"%s\"", beego.AppConfig.String("redisEndpoint")))
		content = setAppConfValue(content, "issuer", casdoorEndpoint)
		content = setAppConfValue(content, "casdoorEndpoint", casdoorEndpoint)
		content = setAppConfValue(content, "clientId", beego.AppConfig.String("clientIdPrefix")+shortName)
		content = setAppConfValue(content, "clientSecret", beego.AppConfig.String("clientSecretPrefix")+shortName)
		content = setAppConfValue(content, "casdoorOrganization", fmt.Sprintf("\"%s\"", shortName))
		content = setAppConfValue(content, "casdoorApplication", fmt.Sprintf("\"app-%s\"", shortName))
	} else if strings.HasPrefix(name, "opendata_customer_") {
		shortName := strings.ReplaceAll(name, "opendata_customer_", "od")
		if orgName != "" {
			shortName = orgName
		}

		casdoorEndpoint := strings.ReplaceAll(beego.AppConfig.String("casdoorEndpoint"), "my.", "od.")
		content = setAppConfValue(content, "httpport", fmt.Sprintf("%d", 50000+i))
		content = setAppConfValue(content, "dataSourceName", fmt.Sprintf("%s:%s@tcp(%s:3306)/", beego.AppConfig.String("dbUser"), beego.AppConfig.String("dbPass"), beego.AppConfig.String("dbHost")))
		content = setAppConfValue(content, "dbName", name)
		content = setAppConfValue(content, "redisEndpoint", fmt.Sprintf("\"%s\"", beego.AppConfig.String("redisEndpoint")))
		content = setAppConfValue(content, "casdoorEndpoint", casdoorEndpoint)
		content = setAppConfValue(content, "clientId", beego.AppConfig.String("clientIdPrefix")+shortName)
		content = setAppConfValue(content, "clientSecret", beego.AppConfig.String("clientSecretPrefix")+shortName)
		content = setAppConfValue(content, "casdoorOrganization", fmt.Sprintf("\"%s\"", shortName))
		content = setAppConfValue(content, "casdoorApplication", fmt.Sprintf("\"app-%s\"", shortName))
	} else if strings.HasPrefix(name, "openct_customer_") {
		shortName := strings.ReplaceAll(name, "openct_customer_", "oct")
		if orgName != "" {
			shortName = orgName
		}

		casdoorEndpoint := strings.ReplaceAll(beego.AppConfig.String("casdoorEndpoint"), "my.", "oct.")
		content = setAppConfValue(content, "httpport", fmt.Sprintf("%d", 60000+i))
		content = setAppConfValue(content, "dataSourceName", fmt.Sprintf("%s:%s@tcp(%s:3306)/", beego.AppConfig.String("dbUser"), beego.AppConfig.String("dbPass"), beego.AppConfig.String("dbHost")))
		content = setAppConfValue(content, "dbName", name)
		content = setAppConfValue(content, "redisEndpoint", fmt.Sprintf("\"%s\"", beego.AppConfig.String("redisEndpoint")))
		content = setAppConfValue(content, "casdoorEndpoint", casdoorEndpoint)
		content = setAppConfValue(content, "clientId", beego.AppConfig.String("clientIdPrefix")+shortName)
		content = setAppConfValue(content, "clientSecret", beego.AppConfig.String("clientSecretPrefix")+shortName)
		content = setAppConfValue(content, "casdoorOrganization", fmt.Sprintf("\"%s\"", shortName))
		content = setAppConfValue(content, "casdoorApplication", fmt.Sprintf("\"app-%s\"", shortName))
	} else if strings.HasPrefix(name, "casos_customer_") {
		shortName := strings.ReplaceAll(name, "casos_customer_", "cas")
		if orgName != "" {
			shortName = orgName
		}

		casdoorEndpoint := strings.ReplaceAll(beego.AppConfig.String("casdoorEndpoint"), "my.", "cas.")
		content = setAppConfValue(content, "httpport", fmt.Sprintf("%d", 20080+i))
		content = setAppConfValue(content, "driverName", "mysql")
		content = setAppConfValue(content, "dataSourceName", fmt.Sprintf("%s:%s@tcp(%s:3306)/", beego.AppConfig.String("dbUser"), beego.AppConfig.String("dbPass"), beego.AppConfig.String("dbHost")))
		content = setAppConfValue(content, "dbName", name)
		content = setAppConfValue(content, "casdoorEndpoint", casdoorEndpoint)
		content = setAppConfValue(content, "clientId", beego.AppConfig.String("clientIdPrefix")+shortName)
		content = setAppConfValue(content, "clientSecret", beego.AppConfig.String("clientSecretPrefix")+shortName)
		content = setAppConfValue(content, "casdoorOrganization", fmt.Sprintf("\"%s\"", shortName))
		content = setAppConfValue(content, "casdoorApplication", fmt.Sprintf("\"app-%s\"", shortName))
	} else {
		content = strings.ReplaceAll(content, "httpport = 8000", fmt.Sprintf("httpport = %d", 30000+i))
		content = strings.ReplaceAll(content, "123456", beego.AppConfig.String("dbPass"))
		content = strings.ReplaceAll(content, "dbName = casdoor", fmt.Sprintf("dbName = %s", strings.Replace(name, "_00", "_", 1)))
		content = strings.ReplaceAll(content, "redisEndpoint =", "redisEndpoint = \"localhost:6379\"")
		content = strings.ReplaceAll(content, "socks5Proxy = \"127.0.0.1:10808\"", "socks5Proxy =")
		if initAdminPassword != "" {
			content = setInitAdminPassword(content, initAdminPassword)
		}
	}

	util.WriteStringToPath(content, confPath)
}

func updateBatFile(name string) (bool, error) {
	batPath := getBatPath(name)
	err := ensureFileFolderExists(filepath.Dir(batPath))
	if err != nil {
		return false, err
	}

	if util.FileExist(batPath) {
		return true, nil
	}

	fmt.Printf("updateBatFile(): [%s]\n", name)

	util.WriteStringToPath(getGoRunBatContent(name), batPath)
	return false, nil
}

func getGoRunBatContent(name string) string {
	return fmt.Sprintf("cd %s\ngo run main.go", GetRepoPath(name))
}

var shortcutFailureMap = map[string]time.Time{}

// ensureShortcutFile creates the shortcut in the Startup folder that starts the site at boot. A virus scanner (360)
// often blocks creating it from a script; the site does not need it, as startProcess() runs the bat without it and
// CasWAF starts the sites that are not running after a reboot, so a failure is only logged and retried an hour later.
func ensureShortcutFile(name string) {
	if util.FileExist(getShortcutPath(name)) {
		return
	}
	if t, ok := shortcutFailureMap[name]; ok && time.Since(t) < time.Hour {
		return
	}

	err := updateShortcutFile(name)
	if err != nil {
		shortcutFailureMap[name] = time.Now()
		fmt.Printf("updateShortcutFile(): [%s] %s, the site is started from its bat\n", name, err.Error())
		return
	}
	delete(shortcutFailureMap, name)
}

func updateShortcutFile(name string) error {
	fmt.Printf("updateShortcutFile(): [%s]\n", name)

	cmd := exec.Command("powershell", fmt.Sprintf("$s=(New-Object -COM WScript.Shell).CreateShortcut('%s');$s.TargetPath='%s';$s.Save()", getShortcutPath(name), getBatPath(name)))
	return cmd.Run()
}
