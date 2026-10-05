// Copyright 2025 The casbin Authors. All Rights Reserved.
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
	"time"

	"github.com/casbin/caswaf/util"
	"github.com/xorm-io/core"
)

type Node struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	DisplayName string `xorm:"varchar(100)" json:"displayName"`

	Tag         string `xorm:"varchar(100)" json:"tag"`
	ClientIp    string `xorm:"varchar(100)" json:"clientIp"`
	UpgradeMode string `xorm:"varchar(100)" json:"upgradeMode"`
	RunMode     string `xorm:"varchar(100)" json:"runMode"`
	SelfUpdate  bool   `json:"selfUpdate"`
}

func GetGlobalNodes() ([]*Node, error) {
	nodes := []*Node{}
	err := ormer.Engine.Asc("owner").Desc("created_time").Find(&nodes)
	return nodes, err
}

func GetNodes(owner string) ([]*Node, error) {
	nodes := []*Node{}
	err := ormer.Engine.Asc("tag").Desc("created_time").Find(&nodes, &Node{Owner: owner})
	if err != nil {
		return nil, err
	}

	return nodes, nil
}

func getNode(owner string, name string) (*Node, error) {
	node := Node{Owner: owner, Name: name}
	existed, err := ormer.Engine.Get(&node)
	if err != nil {
		return nil, err
	}

	if existed {
		return &node, nil
	}
	return nil, nil
}

func GetNode(id string) (*Node, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	return getNode(owner, name)
}

func UpdateNode(id string, node *Node) (bool, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	if n, err := getNode(owner, name); err != nil {
		return false, err
	} else if n == nil {
		return false, nil
	}

	_, err := ormer.Engine.ID(core.PK{owner, name}).AllCols().Update(node)
	if err != nil {
		return false, err
	}

	return true, nil
}

func AddNode(node *Node) (bool, error) {
	affected, err := ormer.Engine.Insert(node)
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func DeleteNode(node *Node) (bool, error) {
	affected, err := ormer.Engine.ID(core.PK{node.Owner, node.Name}).Delete(&Node{})
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func (node *Node) GetId() string {
	return fmt.Sprintf("%s/%s", node.Owner, node.Name)
}

func GetNodeCount(owner, field, value string) (int64, error) {
	session := GetSession(owner, -1, -1, field, value, "", "")
	return session.Count(&Node{})
}

func GetPaginationNodes(owner string, offset, limit int, field, value, sortField, sortOrder string) ([]*Node, error) {
	nodes := []*Node{}
	session := GetSession(owner, offset, limit, field, value, sortField, sortOrder)
	err := session.Where("owner = ? or owner = ?", "admin", owner).Find(&nodes)
	if err != nil {
		return nodes, err
	}

	return nodes, nil
}

func (node *Node) IsBinaryRunMode() bool {
	return node.RunMode == "Binary"
}

// ShouldAllowUpgrade checks if upgrade is allowed based on node's upgrade mode
func (node *Node) ShouldAllowUpgrade() bool {
	if node.UpgradeMode == "" || node.UpgradeMode == "At Any Time" {
		return true
	}

	if node.UpgradeMode == "No Upgrade" {
		return false
	}

	if node.UpgradeMode == "Half A Hour" {
		// Check if current time is in the 23:00-23:30 GMT+8 window
		// GMT+8 is 8 hours ahead of UTC
		location := time.FixedZone("GMT+8", 8*60*60)
		now := time.Now().In(location)

		hour := now.Hour()
		minute := now.Minute()

		// Allow upgrade if time is between 23:00 and 23:30
		if hour == 23 && minute < 30 {
			return true
		}
		return false
	}

	// Default to allowing upgrade for unknown modes
	return true
}
