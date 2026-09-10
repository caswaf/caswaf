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
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/casbin/caswaf/util"
)

// gitTimeout prevents a git command that waits for input from blocking the site monitor loop forever
const gitTimeout = 10 * time.Minute

func runGitCommand(path string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = path
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_EDITOR=true")

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return out.String(), fmt.Errorf("git %s: timed out after %s", strings.Join(args, " "), gitTimeout)
	}
	if err != nil {
		return out.String(), fmt.Errorf("git %s: %s, output: %s", strings.Join(args, " "), err.Error(), strings.TrimSpace(stderr.String()))
	}

	return out.String(), nil
}

func gitClone(repoUrl string, path string) error {
	fmt.Printf("gitClone(): [%s]\n", path)

	_, err := runGitCommand("", "clone", repoUrl, path)
	return err
}

func GitDiff(path string) (string, error) {
	return runGitCommand(path, "diff")
}

func gitDiffHead(path string) (string, error) {
	return runGitCommand(path, "diff", "HEAD")
}

func gitApply(path string, patch string) error {
	fmt.Printf("gitApply(): [%s]\n", path)

	tmpFile, err := ioutil.TempFile("", "patch")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(patch)
	if err != nil {
		return err
	}

	err = tmpFile.Close()
	if err != nil {
		return err
	}

	_, err = runGitCommand(path, "apply", tmpFile.Name())
	return err
}

func gitGetLatestCommitHash(path string) (string, error) {
	out, err := runGitCommand(path, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(out), nil
}

func gitHasUnmergedFiles(path string) (bool, error) {
	out, err := runGitCommand(path, "ls-files", "--unmerged")
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(out) != "", nil
}

func gitRestoreCommit(path string, hash string, localDiff string) error {
	_, err := runGitCommand(path, "reset", "--hard", hash)
	if err != nil {
		return err
	}

	if localDiff == "" {
		return nil
	}

	return gitApply(path, localDiff)
}

// gitRecoverFromConflict cleans up the leftovers of a failed rebase, merge or autostash, they block every later "git pull"
func gitRecoverFromConflict(path string) error {
	gitDir := filepath.Join(path, ".git")
	if util.FileExist(filepath.Join(gitDir, "rebase-merge")) || util.FileExist(filepath.Join(gitDir, "rebase-apply")) {
		_, err := runGitCommand(path, "rebase", "--abort")
		if err != nil {
			return err
		}
	} else if util.FileExist(filepath.Join(gitDir, "MERGE_HEAD")) {
		_, err := runGitCommand(path, "merge", "--abort")
		if err != nil {
			return err
		}
	}

	conflicted, err := gitHasUnmergedFiles(path)
	if err != nil {
		return err
	}
	if !conflicted {
		return nil
	}

	fmt.Printf("gitRecoverFromConflict(): [%s]\n", path)

	// git saved the local changes as the latest stash entry, they apply cleanly again on the commit that stash was taken on
	stashBase, err := runGitCommand(path, "rev-parse", "stash@{0}^")
	if err == nil {
		_, err = runGitCommand(path, "reset", "--hard", strings.TrimSpace(stashBase))
		if err != nil {
			return err
		}

		_, err = runGitCommand(path, "stash", "pop")
		if err == nil {
			return nil
		}
	}

	// the stash is unusable, so only drop the unmerged entries and keep the files, nothing is lost
	_, err = runGitCommand(path, "reset")
	return err
}

func gitPull(path string) (bool, error) {
	err := gitRecoverFromConflict(path)
	if err != nil {
		return false, err
	}

	oldHash, err := gitGetLatestCommitHash(path)
	if err != nil {
		return false, err
	}

	localDiff, err := gitDiffHead(path)
	if err != nil {
		return false, err
	}

	out, err := runGitCommand(path, "pull", "--rebase", "--autostash")
	if err != nil {
		return false, err
	}

	// "git pull --rebase --autostash" exits with 0 but leaves the index unmerged when re-applying the local changes conflicts
	conflicted, err := gitHasUnmergedFiles(path)
	if err != nil {
		return false, err
	}
	if conflicted {
		err = gitRestoreCommit(path, oldHash, localDiff)
		if err != nil {
			return false, err
		}

		return false, fmt.Errorf("gitPull(): the local changes of [%s] conflict with the upstream code and need to be merged manually", path)
	}

	newHash, err := gitGetLatestCommitHash(path)
	if err != nil {
		return false, err
	}

	affected := oldHash != newHash

	if affected {
		fmt.Printf("gitPull(): [%s]\n", path)
		fmt.Printf("Output: %s\n", out)
		fmt.Printf("Affected: [%s] -> [%s]\n", oldHash, newHash)
	}

	return affected, nil
}

func gitWebBuild(path string) error {
	webDir := filepath.Join(path, "web")
	fmt.Printf("gitWebBuild(): [%s]\n", webDir)

	useNpm := false
	if _, err := os.Stat(filepath.Join(webDir, "yarn.lock")); os.IsNotExist(err) {
		if _, err2 := os.Stat(filepath.Join(webDir, "package-lock.json")); err2 == nil {
			useNpm = true
		}
	}

	if useNpm {
		if err := runCmd(webDir, "npm", "install"); err != nil {
			return err
		}
		return runCmd(webDir, "npm", "run", "build")
	}

	if err := runCmd(webDir, "yarn", "install"); err != nil {
		return err
	}
	return runCmd(webDir, "yarn", "build")
}
