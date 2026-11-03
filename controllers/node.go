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

package controllers

import (
	"encoding/json"

	"github.com/beego/beego/utils/pagination"
	"github.com/casbin/caswaf/object"
	"github.com/casbin/caswaf/util"
)

func (c *ApiController) GetGlobalNodes() {
	if c.RequireAdmin() {
		return
	}

	nodes, err := object.GetGlobalNodes()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(nodes)
}

func (c *ApiController) GetNodes() {
	if c.RequireAdmin() {
		return
	}

	owner := c.Input().Get("owner")
	if owner == "admin" {
		owner = ""
	}

	limit := c.Input().Get("pageSize")
	page := c.Input().Get("p")
	field := c.Input().Get("field")
	value := c.Input().Get("value")
	sortField := c.Input().Get("sortField")
	sortOrder := c.Input().Get("sortOrder")

	if limit == "" || page == "" {
		nodes, err := object.GetNodes(owner)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}
		c.ResponseOk(nodes)
		return
	}

	limitInt := util.ParseInt(limit)
	count, err := object.GetNodeCount(owner, field, value)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	paginator := pagination.SetPaginator(c.Ctx, limitInt, count)
	nodes, err := object.GetPaginationNodes(owner, paginator.Offset(), limitInt, field, value, sortField, sortOrder)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(nodes, paginator.Nums())
}

func (c *ApiController) GetNode() {
	if c.RequireAdmin() {
		return
	}

	id := c.Input().Get("id")

	node, err := object.GetNode(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(node)
}

func (c *ApiController) UpdateNode() {
	if c.RequireAdmin() {
		return
	}

	id := c.Input().Get("id")

	var node object.Node
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &node)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.Data["json"] = wrapActionResponse(object.UpdateNode(id, &node))
	c.ServeJSON()
}

func (c *ApiController) AddNode() {
	if c.RequireAdmin() {
		return
	}

	var node object.Node
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &node)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.Data["json"] = wrapActionResponse(object.AddNode(&node))
	c.ServeJSON()
}

func (c *ApiController) DeleteNode() {
	if c.RequireAdmin() {
		return
	}

	var node object.Node
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &node)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.Data["json"] = wrapActionResponse(object.DeleteNode(&node))
	c.ServeJSON()
}
