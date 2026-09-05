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

package object

import (
	"fmt"
	"sync"
	"time"

	"github.com/casbin/caswaf/util"
)

var (
	siteUpdateMap = map[string]string{}
	lock          = &sync.Mutex{}
)

// checkSite calls fn for one site while holding the lock. The lock is released
// via defer and a panic inside fn is turned into an error, so that a broken
// site will neither leak the lock nor stop the other sites from being checked.
func checkSite(fn func() error) (err error) {
	lock.Lock()
	defer lock.Unlock()

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("recovered from panic: %v", r)
		}
	}()

	return fn()
}

func monitorSiteNodes() error {
	sites, err := GetGlobalSites()
	if err != nil {
		return err
	}

	for _, site := range sites {
		//updatedTime, ok := siteUpdateMap[site.GetId()]
		//if ok && updatedTime != "" && updatedTime == site.UpdatedTime {
		//	continue
		//}

		err = checkSite(site.checkNodes)
		if err != nil {
			fmt.Printf("[%s] monitorSiteNodes() error, site = %s: %v\n", util.GetCurrentTime(), site.GetId(), err)
			continue
		}

		siteUpdateMap[site.GetId()] = site.UpdatedTime
	}

	return nil
}

func monitorSiteCerts() error {
	sites, err := GetGlobalSites()
	if err != nil {
		return err
	}

	for _, site := range sites {
		//updatedTime, ok := siteUpdateMap[site.GetId()]
		//if ok && updatedTime != "" && updatedTime == site.UpdatedTime {
		//	continue
		//}

		err = checkSite(site.checkCerts)
		if err != nil {
			fmt.Printf("[%s] monitorSiteCerts() error, site = %s: %v\n", util.GetCurrentTime(), site.GetId(), err)
			continue
		}

		siteUpdateMap[site.GetId()] = site.UpdatedTime
	}

	return nil
}

func monitorSitesOnce() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[%s] Recovered from monitorSitesOnce() panic: %v\n", util.GetCurrentTime(), r)
		}
	}()

	err := refreshSiteMap()
	if err != nil {
		fmt.Println(err)
		return
	}

	err = refreshRuleMap()
	if err != nil {
		fmt.Println(err)
		return
	}

	err = monitorSiteNodes()
	if err != nil {
		fmt.Println(err)
		return
	}

	err = monitorSiteCerts()
	if err != nil {
		fmt.Println(err)
		return
	}

	startHealthCheckLoop()
}

func StartMonitorSitesLoop() {
	fmt.Printf("StartMonitorSitesLoop() Start!\n\n")
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[%s] Recovered from StartMonitorSitesLoop() panic: %v\n", util.GetCurrentTime(), r)
				StartMonitorSitesLoop()
			}
		}()

		for {
			monitorSitesOnce()

			time.Sleep(5 * time.Second)
		}
	}()
}
