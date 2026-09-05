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

func uploadFolder(provider storage.StorageProvider, buildDir string, relDir string, filenames []string, rootDir string) (string, error) {
	domainUrl := ""

	path := filepath.Join(buildDir, relDir)
	for _, filename := range filenames {
		data, err := os.ReadFile(filepath.Join(path, filename))
		if err != nil {
			return "", err
		}
		fileBuffer := bytes.NewBuffer(data)

		objectKey := strings.ReplaceAll(filepath.Join(relDir, filename), "\\", "/")
		fileUrl, err := provider.PutObject("Built-in-Untracked", "", objectKey, fileBuffer)
		if err != nil {
			return "", err
		}

		index := strings.Index(fileUrl, "/"+rootDir)
		if index == -1 {
			return "", fmt.Errorf("uploadFolder() error, fileUrl should contain \"/%s/\", fileUrl = %s", rootDir, fileUrl)
		}

		domainUrl = fileUrl[:index+len("/"+rootDir)] + "/"
		fmt.Printf("uploadFolder(): [/%s] -> [%s]\n", objectKey, fileUrl)
	}

	return domainUrl, nil
}

func updateHtml(domainUrl string, buildDir string, rootDir string) {
	htmlPath := filepath.Join(buildDir, "index.html")
	html := util.ReadStringFromPath(htmlPath)

	html = strings.Replace(html, fmt.Sprintf("\"/%s/", rootDir), fmt.Sprintf("\"%s", domainUrl), -1)
	util.WriteStringToPath(html, htmlPath)

	fmt.Printf("updateHtml(): index.html content:\n%s\n%s\n%s\n", strings.Repeat("=", 80), html, strings.Repeat("=", 80))
}

// uploadCraCdn uploads the CDN files of a Create React App style build, whose
// JS and CSS files are located in web/build/static/js and web/build/static/css.
func uploadCraCdn(provider storage.StorageProvider, buildDir string, siteName string) (string, error) {
	domainUrl := ""
	for _, folder := range []string{"js", "css"} {
		relDir := filepath.Join("static", folder)

		filenames, err := util.ListFiles(filepath.Join(buildDir, relDir))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}

		folderUrl, err := uploadFolder(provider, buildDir, relDir, filterFiles(filenames, folder, siteName), "static")
		if err != nil {
			return "", err
		}

		if folder == "js" {
			domainUrl = folderUrl
		}
	}
	return domainUrl, nil
}

// uploadViteCdn uploads the CDN files of a Vite style build, whose JS, CSS and
// other assets are all located in web/build/assets.
func uploadViteCdn(provider storage.StorageProvider, buildDir string) (string, error) {
	filenames, err := util.ListFiles(filepath.Join(buildDir, "assets"))
	if err != nil {
		return "", err
	}

	return uploadFolder(provider, buildDir, "assets", filenames, "assets")
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

	rootDir := ""
	domainUrl := ""
	if util.FileExist(filepath.Join(buildDir, "static", "js")) || util.FileExist(filepath.Join(buildDir, "static", "css")) {
		rootDir = "static"
		domainUrl, err = uploadCraCdn(provider, buildDir, siteName)
	} else if util.FileExist(filepath.Join(buildDir, "assets")) {
		rootDir = "assets"
		domainUrl, err = uploadViteCdn(provider, buildDir)
	} else {
		return fmt.Errorf("gitUploadCdn() error, neither [%s] nor [%s] exists", filepath.Join(buildDir, "static"), filepath.Join(buildDir, "assets"))
	}
	if err != nil {
		return err
	}

	if domainUrl == "" {
		return fmt.Errorf("gitUploadCdn() error, no CDN file is uploaded in folder: %s", filepath.Join(buildDir, rootDir))
	}

	updateHtml(domainUrl, buildDir, rootDir)
	return nil
}
