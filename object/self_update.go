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

package object

import (
	"fmt"
	"os"
	"time"

	"github.com/beego/beego"
	"github.com/casbin/caswaf/run"
	"github.com/casbin/caswaf/util"
)

const selfUpdateInterval = 5 * time.Minute

var selfUpdateLastError = ""

func selfUpdateOnce() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[%s] Recovered from selfUpdateOnce() panic: %v\n", util.GetCurrentTime(), r)
		}
	}()

	// only the nodes turned on in the DB update themselves, never someone else's CasWAF, and in the same
	// upgrade window as the sites of the node
	hostname := util.GetHostname()
	node, err := getNode("admin", hostname)
	if err != nil || node == nil || !node.SelfUpdate || !node.ShouldAllowUpgrade() {
		return
	}

	// a CasWAF that runs as a site of another CasWAF on this node is pulled and restarted by that one
	site, err := getSite("admin", beego.AppConfig.String("dbName"))
	if err != nil {
		return
	}
	if site != nil {
		for _, item := range site.Nodes {
			if item.Name == hostname {
				return
			}
		}
	}

	ready, err := run.PrepareSelfUpdate()
	if err != nil {
		// a node without access to GitHub fails the same way every time, print it once
		if err.Error() != selfUpdateLastError {
			fmt.Printf("[%s] selfUpdateOnce() error: %v\n", util.GetCurrentTime(), err)
			selfUpdateLastError = err.Error()
		}
		return
	}
	selfUpdateLastError = ""
	if !ready {
		return
	}

	// wait for the site round in progress, so that no repo is left half pulled or built, and never release
	// the lock as the process exits
	lock.Lock()
	err = run.RestartSelf()
	if err != nil {
		lock.Unlock()
		fmt.Printf("[%s] selfUpdateOnce() error: %v\n", util.GetCurrentTime(), err)
		return
	}
	os.Exit(0)
}

// StartSelfUpdateLoop pulls new CasWAF code from GitHub and restarts CasWAF on it
func StartSelfUpdateLoop() {
	if !run.InitSelfUpdate() {
		return
	}

	go func() {
		for {
			time.Sleep(selfUpdateInterval)
			selfUpdateOnce()
		}
	}()
}
