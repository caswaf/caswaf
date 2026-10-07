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

//go:build !skipCi
// +build !skipCi

package run

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beego/beego"
	"github.com/casbin/caswaf/casdoor"
)

func TestGitGetDiff(t *testing.T) {
	err := beego.LoadAppConfig("ini", "../conf/app.conf")
	if err != nil {
		panic(err)
	}

	//diff := GitDiff("F:/github_repos/casdoor")
	//println(diff)

	pid, err := CreateRepo("casdoor_test", true, "", "", "", "", true, false)
	if err != nil {
		panic(err)
	}

	println(pid)
}

func TestUploadCdn(t *testing.T) {
	// object.InitConfig()
	casdoor.InitCasdoorConfig()

	err := gitUploadCdn("provider_storage_aliyun_oss", "casdoor")
	if err != nil {
		panic(err)
	}
}

func TestUpdatePort(t *testing.T) {
	appDir := beego.AppConfig.String("appDir")
	err := filepath.Walk(appDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			panic(err)
		}

		if strings.HasSuffix(path, "/conf/app.conf") {
			var content []byte
			content, err = os.ReadFile(path)
			if err != nil {
				panic(err)
			}

			newContent := strings.Replace(string(content), "(localhost:3306)", "(localhost:3306)", -1)

			err = ioutil.WriteFile(path, []byte(newContent), info.Mode())
			if err != nil {
				panic(err)
			}

			fmt.Printf("Successfully updated: %s\n", path)
		}

		return nil
	})
	if err != nil {
		panic(err)
	}
}
