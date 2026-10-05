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
	"strings"

	"github.com/casbin/caswaf/util"
)

func isThinSiteCandidate(siteName string) bool {
	return strings.HasPrefix(siteName, "casdoor_customer_") && getNameIndex(siteName) != 0
}

func getSharedSiteName(siteName string) string {
	return getOriginalName(siteName)
}

func hasSharedCode(siteName string) bool {
	path := GetRepoPath(getSharedSiteName(siteName))
	return util.FileExist(filepath.Join(path, ".git")) && util.FileExist(filepath.Join(path, "main.go"))
}

func IsThinSite(siteName string) bool {
	path := GetRepoPath(siteName)
	return isThinSiteCandidate(siteName) && util.FileExist(path) && !util.FileExist(filepath.Join(path, ".git"))
}

func GetCodePath(siteName string) string {
	if IsThinSite(siteName) {
		return GetRepoPath(getSharedSiteName(siteName))
	}
	return GetRepoPath(siteName)
}

func checkSharedCode(siteName string) error {
	if IsThinSite(siteName) && !hasSharedCode(siteName) {
		return fmt.Errorf("the thin site needs the code of the site: %s at %s", getSharedSiteName(siteName), GetRepoPath(getSharedSiteName(siteName)))
	}
	return nil
}

func createThinSite(siteName string) error {
	fmt.Printf("createThinSite(): [%s]\n", siteName)

	content, err := runGitCommand(GetRepoPath(getSharedSiteName(siteName)), "show", "HEAD:conf/app.conf")
	if err != nil {
		return err
	}

	confPath := getCodeAppConfPath(siteName)
	err = os.MkdirAll(filepath.Dir(confPath), os.ModePerm)
	if err != nil {
		return err
	}

	return os.WriteFile(confPath, []byte(content), 0644)
}
