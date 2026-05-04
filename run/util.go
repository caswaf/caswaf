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
	"strings"

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

func updateAppConfFile(name string, i int, orgName string) {
	fmt.Printf("updateAppConfFile(): [%s]\n", name)
	confPath := getCodeAppConfPath(name)
	content := util.ReadStringFromPath(confPath)

	if strings.HasPrefix(name, "casibase_customer_") {
		shortName := strings.ReplaceAll(name, "casibase_customer_", "cbc")
		if orgName != "" {
			shortName = orgName
		}

		content = strings.ReplaceAll(content, "httpport = 14000", fmt.Sprintf("httpport = %d", 40000+i))
		content = strings.ReplaceAll(content, "root", beego.AppConfig.String("dbUser"))
		content = strings.ReplaceAll(content, "123456", beego.AppConfig.String("dbPass"))
		content = strings.ReplaceAll(content, "localhost:3306", fmt.Sprintf("%s:3306", beego.AppConfig.String("dbHost")))
		content = strings.ReplaceAll(content, "dbName = casibase", fmt.Sprintf("dbName = %s", name))
		content = strings.ReplaceAll(content, "redisEndpoint =", fmt.Sprintf("redisEndpoint = \"%s\"", beego.AppConfig.String("redisEndpoint")))
		content = strings.ReplaceAll(content, "disablePreviewMode = false", "disablePreviewMode = true")
		content = strings.ReplaceAll(content, "casdoorEndpoint = https://door.casdoor.com", fmt.Sprintf("casdoorEndpoint = %s", strings.ReplaceAll(beego.AppConfig.String("casdoorEndpoint"), "my.", "cbc.")))
		content = strings.ReplaceAll(content, "clientId = af6b5aa958822fb9dc33", fmt.Sprintf("clientId = %s", beego.AppConfig.String("clientIdPrefix")+shortName))
		content = strings.ReplaceAll(content, "clientSecret = 8bc3010c1c951c8d876b1f311a901ff8deeb93bc", fmt.Sprintf("clientSecret = %s", beego.AppConfig.String("clientSecretPrefix")+shortName))
		content = strings.ReplaceAll(content, "casdoorOrganization = \"casbin\"", fmt.Sprintf("casdoorOrganization = \"%s\"", shortName))
		content = strings.ReplaceAll(content, "casdoorApplication = \"app-casibase\"", fmt.Sprintf("casdoorApplication = \"%s\"", fmt.Sprintf("app-%s", shortName)))
		content = strings.ReplaceAll(content, "isLocalIpDb = false", "isLocalIpDb = true")
		content = strings.ReplaceAll(content, "providerDbName = \"\"", "providerDbName = \"casibase_casbin\"")
	} else if strings.HasPrefix(name, "opendata_customer_") {
		shortName := strings.ReplaceAll(name, "opendata_customer_", "od")
		if orgName != "" {
			shortName = orgName
		}

		content = strings.ReplaceAll(content, "httpport = 14000", fmt.Sprintf("httpport = %d", 50000+i))
		content = strings.ReplaceAll(content, "root", beego.AppConfig.String("dbUser"))
		content = strings.ReplaceAll(content, "123456", beego.AppConfig.String("dbPass"))
		content = strings.ReplaceAll(content, "localhost:3306", fmt.Sprintf("%s:3306", beego.AppConfig.String("dbHost")))
		content = strings.ReplaceAll(content, "dbName = opendata", fmt.Sprintf("dbName = %s", name))
		content = strings.ReplaceAll(content, "redisEndpoint =", fmt.Sprintf("redisEndpoint = \"%s\"", beego.AppConfig.String("redisEndpoint")))
		content = strings.ReplaceAll(content, "casdoorEndpoint = https://door.casdoor.com", fmt.Sprintf("casdoorEndpoint = %s", strings.ReplaceAll(beego.AppConfig.String("casdoorEndpoint"), "my.", "od.")))
		content = strings.ReplaceAll(content, "clientId = af6b5aa958822fb9dc33", fmt.Sprintf("clientId = %s", beego.AppConfig.String("clientIdPrefix")+shortName))
		content = strings.ReplaceAll(content, "clientSecret = 8bc3010c1c951c8d876b1f311a901ff8deeb93bc", fmt.Sprintf("clientSecret = %s", beego.AppConfig.String("clientSecretPrefix")+shortName))
		content = strings.ReplaceAll(content, "casdoorOrganization = \"casbin\"", fmt.Sprintf("casdoorOrganization = \"%s\"", shortName))
		content = strings.ReplaceAll(content, "casdoorApplication = \"app-opendata\"", fmt.Sprintf("casdoorApplication = \"%s\"", fmt.Sprintf("app-%s", shortName)))
	} else {
		content = strings.ReplaceAll(content, "httpport = 8000", fmt.Sprintf("httpport = %d", 30000+i))
		content = strings.ReplaceAll(content, "123456", beego.AppConfig.String("dbPass"))
		content = strings.ReplaceAll(content, "dbName = casdoor", fmt.Sprintf("dbName = %s", strings.Replace(name, "_00", "_", 1)))
		content = strings.ReplaceAll(content, "redisEndpoint =", "redisEndpoint = \"localhost:6379\"")
		content = strings.ReplaceAll(content, "socks5Proxy = \"127.0.0.1:10808\"", "socks5Proxy =")
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

	content := fmt.Sprintf("cd %s\ngo run main.go", GetRepoPath(name))
	util.WriteStringToPath(content, batPath)
	return false, nil
}

func updateShortcutFile(name string) error {
	fmt.Printf("updateShortcutFile(): [%s]\n", name)

	cmd := exec.Command("powershell", fmt.Sprintf("$s=(New-Object -COM WScript.Shell).CreateShortcut('%s');$s.TargetPath='%s';$s.Save()", getShortcutPath(name), getBatPath(name)))
	return cmd.Run()
}
