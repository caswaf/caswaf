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
	"os"
	"path/filepath"
	"strings"

	"github.com/casbin/caswaf/storage"
	"github.com/casbin/caswaf/util"
)

func filterFiles(filenames []string, folder string, siteName string) []string {
	res := []string{}
	for _, filename := range filenames {
		if !strings.HasSuffix(filename, folder) {
			continue
		}

		if strings.HasPrefix(siteName, "casdoor") {
			if strings.Contains(filename, ".chunk.js") || strings.Contains(filename, ".chunk.css") {
				continue
			}
		}

		res = append(res, filename)
	}
	return res
}

func uploadFolder(provider storage.StorageProvider, buildDir string, folder string, siteName string) (string, error) {
	domainUrl := ""

	path := filepath.Join(buildDir, "static", folder)
	filenames := util.ListFiles(path)
	filteredFilenames := filterFiles(filenames, folder, siteName)
	for _, filename := range filteredFilenames {
		data, err := os.ReadFile(filepath.Join(path, filename))
		if err != nil {
			return "", err
		}
		fileBuffer := bytes.NewBuffer(data)

		objectKey := strings.ReplaceAll(filepath.Join("static", folder, filename), "\\", "/")
		fileUrl, err := provider.PutObject("Built-in-Untracked", "", objectKey, fileBuffer)
		if err != nil {
			return "", err
		}

		index := strings.Index(fileUrl, "/static")
		if index == -1 {
			return "", fmt.Errorf("uploadFolder() error, fileUrl should contain \"/static/\", fileUrl = %s", fileUrl)
		}

		domainUrl = fileUrl[:index+len("/static")] + "/"
		fmt.Printf("uploadFolder(): [/%s] -> [%s]\n", objectKey, fileUrl)
	}

	return domainUrl, nil
}

func updateHtml(domainUrl string, buildDir string) {
	htmlPath := filepath.Join(buildDir, "index.html")
	html := util.ReadStringFromPath(htmlPath)

	html = strings.Replace(html, "\"/static/", fmt.Sprintf("\"%s", domainUrl), -1)
	util.WriteStringToPath(html, htmlPath)

	fmt.Printf("updateHtml(): index.html content:\n%s\n%s\n%s\n", strings.Repeat("=", 80), html, strings.Repeat("=", 80))
}

func gitUploadCdn(providerName string, siteName string) error {
	if providerName == "" {
		return nil
	}

	fmt.Printf("gitUploadCdn(): [%s]\n", siteName)

	path := GetRepoPath(siteName)
	buildDir := filepath.Join(path, "web/build")

	provider, err := storage.GetStorageProvider(providerName)
	if err != nil {
		return err
	}

	var domainUrl string
	domainUrl, err = uploadFolder(provider, buildDir, "js", siteName)
	if err != nil {
		return err
	}

	_, err = uploadFolder(provider, buildDir, "css", siteName)
	if err != nil {
		return err
	}

	updateHtml(domainUrl, buildDir)
	return nil
}
