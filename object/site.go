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
	"math/rand"
	"strings"
	"time"

	"github.com/casbin/caswaf/run"
	"github.com/casbin/caswaf/util"
	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/xorm-io/core"
	"github.com/xorm-io/xorm"
)

type NodeItem struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Diff     string `json:"diff"`
	Pid      int    `json:"pid"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Provider string `json:"provider"`
}

type Site struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	UpdatedTime string `xorm:"varchar(100)" json:"updatedTime"`
	DisplayName string `xorm:"varchar(100)" json:"displayName"`

	Tag            string      `xorm:"varchar(100)" json:"tag"`
	Domain         string      `xorm:"varchar(100)" json:"domain"`
	OtherDomains   []string    `xorm:"varchar(500)" json:"otherDomains"`
	NeedRedirect   bool        `json:"needRedirect"`
	RedirectSlash  bool        `json:"redirectSlash"`
	DisableVerbose bool        `json:"disableVerbose"`
	Rules          []string    `xorm:"varchar(500)" json:"rules"`
	EnableAlert    bool        `json:"enableAlert"`
	AlertInterval  int         `json:"alertInterval"`
	AlertTryTimes  int         `json:"alertTryTimes"`
	AlertProviders []string    `xorm:"varchar(500)" json:"alertProviders"`
	Challenges     []string    `xorm:"mediumtext" json:"challenges"`
	Host           string      `xorm:"varchar(100)" json:"host"`
	Port           int         `json:"port"`
	Hosts          []string    `xorm:"varchar(1000)" json:"hosts"`
	SslMode        string      `xorm:"varchar(100)" json:"sslMode"`
	SslCert        string      `xorm:"-" json:"sslCert"`
	PublicIp       string      `xorm:"varchar(100)" json:"publicIp"`
	Node           string      `xorm:"varchar(100)" json:"node"`
	IsSelf         bool        `json:"isSelf"`
	Status         string      `xorm:"varchar(100)" json:"status"`
	Nodes          []*NodeItem `xorm:"mediumtext" json:"nodes"`

	CasdoorApplication string                  `xorm:"varchar(100)" json:"casdoorApplication"`
	ApplicationObj     *casdoorsdk.Application `xorm:"-" json:"applicationObj"`
}

func GetGlobalSites() ([]*Site, error) {
	sites := []*Site{}
	err := ormer.Engine.Desc("created_time").Find(&sites)
	if err != nil {
		return nil, err
	}

	return sites, nil
}

func GetSites(owner string) ([]*Site, error) {
	sites := []*Site{}
	err := ormer.Engine.Asc("tag").Asc("port").Desc("created_time").Find(&sites, &Site{Owner: owner})
	if err != nil {
		return nil, err
	}

	for _, site := range sites {
		err = site.populateCert()
		if err != nil {
			return nil, err
		}
	}

	return sites, nil
}

func getSite(owner string, name string) (*Site, error) {
	site := Site{Owner: owner, Name: name}
	existed, err := ormer.Engine.Get(&site)
	if err != nil {
		return nil, err
	}

	if existed {
		return &site, nil
	}
	return nil, nil
}

func GetSite(id string) (*Site, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	site, err := getSite(owner, name)
	if err != nil {
		return nil, err
	}

	if site != nil {
		err = site.populateCert()
		if err != nil {
			return nil, err
		}
	}

	return site, nil
}

func GetMaskedSite(site *Site, node string) *Site {
	if site == nil {
		return nil
	}

	if site.PublicIp == "(empty)" {
		site.PublicIp = ""
	}

	site.IsSelf = false
	if site.Node == node {
		site.IsSelf = true
	}

	return site
}

func GetMaskedSites(sites []*Site, node string) []*Site {
	for _, site := range sites {
		site = GetMaskedSite(site, node)
	}
	return sites
}

func UpdateSite(id string, site *Site) (bool, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	if s, err := getSite(owner, name); err != nil {
		return false, err
	} else if s == nil {
		return false, nil
	}

	site.UpdatedTime = util.GetCurrentTime()

	_, err := ormer.Engine.ID(core.PK{owner, name}).AllCols().Update(site)
	if err != nil {
		return false, err
	}

	err = refreshSiteMap()
	if err != nil {
		return false, err
	}

	err = site.checkNodes()
	if err != nil {
		return false, err
	}

	return true, nil
}

func UpdateSiteNoRefresh(id string, site *Site, cols ...string) (bool, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	if s, err := getSite(owner, name); err != nil {
		return false, err
	} else if s == nil {
		return false, nil
	}

	session := ormer.Engine.ID(core.PK{owner, name})
	if len(cols) == 0 {
		session = session.AllCols()
	} else {
		session = session.Cols(cols...)
	}

	_, err := session.Update(site)
	if err != nil {
		return false, err
	}

	return true, nil
}

func updateSiteNode(owner string, name string, node *NodeItem) error {
	_, err := ormer.Engine.Transaction(func(session *xorm.Session) (interface{}, error) {
		site := Site{Owner: owner, Name: name}
		existed, err := session.ForUpdate().Get(&site)
		if err != nil || !existed {
			return nil, err
		}

		for _, item := range site.Nodes {
			if item != nil && item.Name == node.Name {
				item.Version = node.Version
				item.Diff = node.Diff
				item.Pid = node.Pid
				item.Status = node.Status
				item.Message = node.Message
			}
		}
		site.UpdatedTime = util.GetCurrentTime()

		return session.ID(core.PK{owner, name}).Cols("nodes", "updated_time").Update(&site)
	})
	return err
}

func (site *Site) removeLocalNode() error {
	hostname := util.GetHostname()
	for _, node := range site.Nodes {
		if node == nil || node.Name != hostname {
			continue
		}

		err := run.RemoveRepo(site.Name)
		if err != nil {
			return err
		}

		return removeSiteNode(site.Owner, site.Name, hostname)
	}

	if len(site.Nodes) == 0 {
		_, err := DeleteSite(site)
		return err
	}
	return nil
}

// stopLocalNode keeps the site's process ended on this node, also after the startup shortcut runs it on reboot
func (site *Site) stopLocalNode() error {
	hostname := util.GetHostname()
	for i, node := range site.Nodes {
		if node == nil || node.Name != hostname {
			continue
		}

		ok := false
		if site.Port != 0 {
			ok, _ = pingUrl(fmt.Sprintf("http://localhost:%d", site.Port))
		}

		msg := node.Message
		if ok || time.Since(siteStopTimeMap[site.GetId()]) >= time.Minute {
			siteStopTimeMap[site.GetId()] = time.Now()
			msg = ""
			err := run.StopRepo(site.Name)
			if err != nil {
				msg = addErrorToMsg(msg, "StopRepo", err)
			}
		}

		if node.Status != "Stopped" || node.Message != msg || node.Pid != 0 {
			site.Nodes[i].Status = "Stopped"
			site.Nodes[i].Message = msg
			site.Nodes[i].Pid = 0
			err := updateSiteNode(site.Owner, site.Name, site.Nodes[i])
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func removeSiteNode(owner string, name string, nodeName string) error {
	empty := false
	_, err := ormer.Engine.Transaction(func(session *xorm.Session) (interface{}, error) {
		site := Site{Owner: owner, Name: name}
		existed, err := session.ForUpdate().Get(&site)
		if err != nil || !existed {
			return nil, err
		}

		nodes := []*NodeItem{}
		for _, item := range site.Nodes {
			if item != nil && item.Name != nodeName {
				nodes = append(nodes, item)
			}
		}
		site.Nodes = nodes
		site.UpdatedTime = util.GetCurrentTime()
		empty = len(nodes) == 0

		if empty {
			return session.ID(core.PK{owner, name}).Delete(&Site{})
		}
		return session.ID(core.PK{owner, name}).Cols("nodes", "updated_time").Update(&site)
	})
	if err != nil {
		return err
	}

	fmt.Printf("removeSiteNode(): removed node %s from site %s/%s, site deleted: %v\n", nodeName, owner, name, empty)
	return refreshSiteMap()
}

func AddSite(site *Site) (bool, error) {
	affected, err := ormer.Engine.Insert(site)
	if err != nil {
		return false, err
	}

	if affected != 0 {
		err = refreshSiteMap()
		if err != nil {
			return false, err
		}
	}

	return affected != 0, nil
}

func DeleteSite(site *Site) (bool, error) {
	affected, err := ormer.Engine.ID(core.PK{site.Owner, site.Name}).Delete(&Site{})
	if err != nil {
		return false, err
	}

	if affected != 0 {
		err = refreshSiteMap()
		if err != nil {
			return false, err
		}
	}

	return affected != 0, nil
}

func (site *Site) GetId() string {
	return fmt.Sprintf("%s/%s", site.Owner, site.Name)
}

func (site *Site) GetChallengeMap() map[string]string {
	m := map[string]string{}
	for _, challenge := range site.Challenges {
		tokens := strings.Split(challenge, ":")
		m[tokens[0]] = tokens[1]
	}
	return m
}

func (site *Site) GetHost() string {
	if len(site.Hosts) != 0 {
		rand.Seed(time.Now().UnixNano())
		return site.Hosts[rand.Intn(len(site.Hosts))]
	}

	if site.Host != "" {
		return site.Host
	}

	if site.Port == 0 {
		return ""
	}

	res := fmt.Sprintf("http://localhost:%d", site.Port)
	return res
}

func addErrorToMsg(msg string, function string, err error) string {
	fmt.Printf("%s(): %s\n", function, err.Error())
	if msg == "" {
		return fmt.Sprintf("%s(): %s", function, err.Error())
	} else {
		return fmt.Sprintf("%s || %s(): %s", msg, function, err.Error())
	}
}

func (site *Site) checkNodes() error {
	if site.Status == "Inactive" {
		return nil
	}

	if site.Status == "Deleting" {
		return site.removeLocalNode()
	}

	if site.Status == "Stopped" {
		return site.stopLocalNode()
	}

	hostname := util.GetHostname()
	for i, node := range site.Nodes {
		if node.Name != hostname {
			continue
		}

		if site.GetHost() == "" {
			continue
		}

		ok, msg := pingUrl(site.GetHost())
		status := "Running"
		if !ok {
			msg = ""
			if node.Pid > 0 {
				var err error
				ok, err = run.IsProcessActive(node.Pid)
				if err != nil {
					msg = addErrorToMsg(msg, "IsProcessActive", err)
				}
			}
			// Also check if the window title exists (process may be starting/compiling)
			if !ok {
				var err error
				ok, err = run.IsWindowTitleActive(site.Name)
				if err != nil {
					msg = addErrorToMsg(msg, "IsWindowTitleActive", err)
				}
			}
		}
		if !ok {
			status = "Stopped"
		}

		diff := ""
		if i != 0 {
			diff = site.Nodes[0].Diff
		}

		orgName := ""
		if len(site.Challenges) > 0 && strings.HasPrefix(site.Challenges[0], "cbc") {
			orgName = site.Challenges[0]
		}

		// Check if upgrade is allowed based on the upgrade mode of the node that
		// actually runs this site, which is the current machine, not site.Node
		shouldUpgrade := true
		useBinary := false
		nodeObj, err := getNode("admin", node.Name)
		if err != nil {
			msg = addErrorToMsg(msg, "GetNode", err)
		} else if nodeObj != nil {
			shouldUpgrade = nodeObj.ShouldAllowUpgrade()
			useBinary = nodeObj.IsBinaryRunMode()
		}

		pid, err := run.CreateRepo(site.Name, !ok, diff, node.Provider, orgName, shouldUpgrade, useBinary)
		if err != nil {
			msg = addErrorToMsg(msg, "CreateRepo", err)
		}

		// the process may still run old code even though the pull above restarted it, e.g. when the new instance
		// exited before taking over the port, so compare the running process with the checked out code every time
		if ok && err == nil && shouldUpgrade {
			var restartedPid int
			var staleMsg string
			restartedPid, staleMsg, err = run.RestartIfStale(site.Name, site.Port, useBinary)
			if err != nil {
				msg = addErrorToMsg(msg, "RestartIfStale", err)
			} else if staleMsg != "" {
				msg = addErrorToMsg(msg, "RestartIfStale", fmt.Errorf("%s", staleMsg))
			}
			if restartedPid != 0 {
				pid = restartedPid
			}
		}

		if pid == 0 {
			pid = node.Pid
		}

		version, err := getSiteVersion(site.Name)
		if err != nil {
			msg = addErrorToMsg(msg, "getSiteVersion", err)
		}

		newDiff := ""
		if !run.IsThinSite(site.Name) {
			newDiff, err = run.GitDiff(run.GetRepoPath(site.Name))
			if err != nil {
				msg = addErrorToMsg(msg, "GitDiff", err)
			}
		}

		if node.Status != status || node.Message != msg || node.Version != version || node.Diff != newDiff || node.Pid != pid {
			site.Nodes[i].Version = version
			site.Nodes[i].Diff = newDiff
			site.Nodes[i].Pid = pid
			site.Nodes[i].Status = status
			site.Nodes[i].Message = msg
			err = updateSiteNode(site.Owner, site.Name, site.Nodes[i])
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func GetSiteCount(owner, field, value string) (int64, error) {
	session := GetSession(owner, -1, -1, field, value, "", "")
	return session.Count(&Site{})
}

func GetPaginationSites(owner string, offset, limit int, field, value, sortField, sortOrder string) ([]*Site, error) {
	sites := []*Site{}
	session := GetSession(owner, offset, limit, field, value, sortField, sortOrder)
	err := session.Where("owner = ? or owner = ?", "admin", owner).Find(&sites)
	if err != nil {
		return sites, err
	}

	return sites, nil
}
