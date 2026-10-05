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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/beego/beego"
	"github.com/casbin/caswaf/util"
)

const (
	binaryBuildTimeout    = 30 * time.Minute
	binaryRetryInterval   = 30 * time.Minute
	binaryBuildingMaxAge  = 2 * 24 * time.Hour
	binaryDirName         = "_bin"
	binaryPointerFileTail = ".exe.txt"
)

var (
	binaryBuildLock    = &sync.Mutex{}
	binaryFailureMap   = map[string]binaryFailure{}
	binaryFailureMutex = &sync.Mutex{}
	// binaries that were built but could not be renamed into place yet, guarded by binaryBuildLock
	binaryBuiltMap = map[string]bool{}
)

type binaryFailure struct {
	time time.Time
	err  error
}

func isSelfSite(siteName string) bool {
	return getMappedName(siteName) == beego.AppConfig.String("dbName")
}

func canRunBinary(siteName string) bool {
	return !isSelfSite(siteName) && util.FileExist(filepath.Join(GetCodePath(siteName), "main.go"))
}

func shouldRunBinary(siteName string, useBinary bool) bool {
	return (useBinary || IsThinSite(siteName)) && canRunBinary(siteName)
}

func getBinaryDir(siteName string) string {
	return filepath.Join(beego.AppConfig.String("appDir"), binaryDirName, getOriginalName(siteName))
}

func getBinaryPointerPath(name string) string {
	name = getMappedName(name)
	return fmt.Sprintf("C:/Users/%s/Desktop/run/%s%s", username, name, binaryPointerFileTail)
}

func getBinaryPath(siteName string) (string, string, error) {
	path := GetCodePath(siteName)
	hash, err := gitGetLatestCommitHash(path)
	if err != nil {
		return "", "", err
	}

	fileName := fmt.Sprintf("%s_%s", getOriginalName(siteName), hash[:12])
	if IsThinSite(siteName) {
		return filepath.Join(getBinaryDir(siteName), fileName+".exe"), hash, nil
	}

	goDiff, err := runGitCommand(path, "diff", "HEAD", "--", "*.go", "go.mod", "go.sum")
	if err != nil {
		return "", "", err
	}
	if goDiff != "" {
		sum := sha256.Sum256([]byte(goDiff))
		fileName = fmt.Sprintf("%s_%s", fileName, hex.EncodeToString(sum[:])[:8])
	}

	return filepath.Join(getBinaryDir(siteName), fileName+".exe"), hash, nil
}

func buildBinary(repoPath string, binaryPath string) error {
	err := ensureFileFolderExists(filepath.Dir(binaryPath))
	if err != nil {
		return err
	}

	tmpPath := strings.TrimSuffix(binaryPath, ".exe") + ".building.exe"
	if binaryBuiltMap[binaryPath] && util.FileExist(tmpPath) {
		return renameBuiltBinary(tmpPath, binaryPath)
	}
	delete(binaryBuiltMap, binaryPath)
	_ = os.Remove(tmpPath)

	fmt.Printf("buildBinary(): [%s] -> [%s]\n", repoPath, binaryPath)

	ctx, cancel := context.WithTimeout(context.Background(), binaryBuildTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "build", "-ldflags=-s -w", "-o", tmpPath, ".")
	cmd.Dir = repoPath
	var stderr bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("go build: timed out after %s", binaryBuildTimeout)
	}
	if err != nil {
		_ = os.Remove(tmpPath)
		output := strings.TrimSpace(stderr.String())
		if len(output) > 1000 {
			output = output[len(output)-1000:]
		}
		return fmt.Errorf("go build: %s, output: %s", err.Error(), output)
	}

	binaryBuiltMap[binaryPath] = true
	return renameBuiltBinary(tmpPath, binaryPath)
}

// A virus scanner (e.g. 360 on sg-machine) keeps a freshly built exe open for a while, so the rename is retried,
// and if it still fails the built file is kept for the next round instead of being built again.
func renameBuiltBinary(tmpPath string, binaryPath string) error {
	var err error
	for i := 0; i < 30; i++ {
		err = os.Rename(tmpPath, binaryPath)
		if err == nil {
			delete(binaryBuiltMap, binaryPath)
			return nil
		}
		time.Sleep(time.Second)
	}
	return err
}

// ensureBinary builds the site's code once per commit into a binary shared by all the sites of the same repo,
// and points the site's bat to it
func ensureBinary(siteName string) (string, error) {
	binaryPath, hash, err := getBinaryPath(siteName)
	if err != nil {
		return "", err
	}

	binaryBuildLock.Lock()
	defer binaryBuildLock.Unlock()

	if !util.FileExist(binaryPath) {
		binaryFailureMutex.Lock()
		failure, ok := binaryFailureMap[binaryPath]
		binaryFailureMutex.Unlock()
		if ok && time.Since(failure.time) < binaryRetryInterval {
			return "", fmt.Errorf("%s (failed at %s)", failure.err.Error(), failure.time.Format(time.RFC3339))
		}

		srcPath := GetCodePath(siteName)
		if IsThinSite(siteName) {
			srcPath, err = preparePristineSource(siteName, hash)
		}
		if err == nil {
			err = buildBinary(srcPath, binaryPath)
		}
		if err != nil {
			if !binaryBuiltMap[binaryPath] {
				binaryFailureMutex.Lock()
				binaryFailureMap[binaryPath] = binaryFailure{time: time.Now(), err: err}
				binaryFailureMutex.Unlock()
			}
			return "", err
		}
	}

	pointerPath := getBinaryPointerPath(siteName)
	err = ensureFileFolderExists(filepath.Dir(pointerPath))
	if err != nil {
		return "", err
	}
	content := filepath.FromSlash(binaryPath) + "\r\n"
	if readFileString(pointerPath) != content {
		err = os.WriteFile(pointerPath, []byte(content), 0644)
		if err != nil {
			return "", err
		}
	}

	return binaryPath, nil
}

func getBinaryBatContent(name string) string {
	return fmt.Sprintf("cd /d %s\r\nfor /f \"usebackq delims=\" %%%%i in (\"%s\") do \"%%%%i\"\r\n",
		filepath.FromSlash(GetRepoPath(name)), filepath.FromSlash(getBinaryPointerPath(name)))
}

func readFileString(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func isBinaryBat(name string) bool {
	return readFileString(getBatPath(name)) == getBinaryBatContent(name)
}

// updateBinaryBatFile must not run while the old bat is running: cmd.exe reads a bat line by line,
// so it would go on with the new content after the program in the old bat exits
func updateBinaryBatFile(name string) error {
	batPath := getBatPath(name)
	err := ensureFileFolderExists(filepath.Dir(batPath))
	if err != nil {
		return err
	}

	if isBinaryBat(name) {
		return nil
	}

	fmt.Printf("updateBinaryBatFile(): [%s]\n", name)
	return os.WriteFile(batPath, []byte(getBinaryBatContent(name)), 0644)
}

func getRunningBinaryPath(siteName string) string {
	return strings.TrimSpace(readFileString(getBinaryPointerPath(siteName)))
}

// cleanOldBinaries keeps, for every repo, the binaries the sites point to plus the most recently built unused
// commit (all its variants) for a rollback, and removes the other ones. A ".building.exe" left behind is removed
// once it is older than cutoff.
func cleanOldBinaries(cutoff time.Time) (int, int64, error) {
	binaryRoot := filepath.Join(beego.AppConfig.String("appDir"), binaryDirName)
	if !util.FileExist(binaryRoot) {
		return 0, 0, nil
	}

	// a binary just built is only pointed to after ensureBinary() returns
	binaryBuildLock.Lock()
	defer binaryBuildLock.Unlock()

	usedMap := map[string]bool{}
	pointerPaths, err := filepath.Glob(fmt.Sprintf("C:/Users/%s/Desktop/run/*%s", username, binaryPointerFileTail))
	if err != nil {
		return 0, 0, err
	}
	for _, pointerPath := range pointerPaths {
		usedMap[strings.ToLower(strings.TrimSpace(readFileString(pointerPath)))] = true
	}

	repoDirs, err := filepath.Glob(filepath.Join(binaryRoot, "*"))
	if err != nil {
		return 0, 0, err
	}

	count := 0
	size := int64(0)
	remove := func(binaryPath string, info os.FileInfo) {
		// a binary still run by a process is locked and can't be removed
		if os.Remove(binaryPath) == nil {
			count++
			size += info.Size()
		}
	}

	for _, repoDir := range repoDirs {
		binaryPaths, err := filepath.Glob(filepath.Join(repoDir, "*.exe"))
		if err != nil {
			return count, size, err
		}

		type unusedBinary struct {
			path   string
			commit string
			info   os.FileInfo
		}
		unused := []unusedBinary{}
		keepCommit := ""
		keepTime := time.Time{}
		for _, binaryPath := range binaryPaths {
			info, err := os.Stat(binaryPath)
			if err != nil {
				continue
			}

			if strings.HasSuffix(binaryPath, ".building.exe") {
				if info.ModTime().Before(cutoff) {
					remove(binaryPath, info)
				}
				continue
			}

			if usedMap[strings.ToLower(filepath.FromSlash(binaryPath))] {
				continue
			}

			// <repo>_<commit>[_<diff hash>].exe
			commit := strings.TrimPrefix(strings.TrimSuffix(filepath.Base(binaryPath), ".exe"), filepath.Base(repoDir)+"_")
			commit = strings.SplitN(commit, "_", 2)[0]
			unused = append(unused, unusedBinary{path: binaryPath, commit: commit, info: info})
			if info.ModTime().After(keepTime) {
				keepTime = info.ModTime()
				keepCommit = commit
			}
		}

		for _, binary := range unused {
			if binary.commit != keepCommit {
				remove(binary.path, binary.info)
			}
		}
	}
	return count, size, nil
}

func switchBatFile(name string, useBinary bool) error {
	if useBinary {
		return updateBinaryBatFile(name)
	}

	fmt.Printf("switchBatFile(): [%s] go run\n", name)
	return os.WriteFile(getBatPath(name), []byte(getGoRunBatContent(name)), 0644)
}
