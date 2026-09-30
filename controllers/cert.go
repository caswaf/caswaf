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

package controllers

import (
	"encoding/json"

	"github.com/beego/beego/utils/pagination"
	"github.com/casbin/caswaf/object"
	"github.com/casbin/caswaf/util"
)

func (c *ApiController) GetGlobalCerts() {
	if c.RequireSignedIn() {
		return
	}

	certs, err := object.GetGlobalCerts()
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(object.GetMaskedCerts(certs))
}

func (c *ApiController) GetCerts() {
	if c.RequireSignedIn() {
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
		certs, err := object.GetCerts(owner)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}

		c.ResponseOk(object.GetMaskedCerts(certs))
	} else {
		limit := util.ParseInt(limit)
		count, err := object.GetCertCount(owner, field, value)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}

		paginator := pagination.SetPaginator(c.Ctx, limit, count)
		certs, err := object.GetPaginationCerts(owner, paginator.Offset(), limit, field, value, sortField, sortOrder)
		if err != nil {
			c.ResponseError(err.Error())
			return
		}

		c.ResponseOk(object.GetMaskedCerts(certs), paginator.Nums())
	}
}

func (c *ApiController) GetCert() {
	if c.RequireSignedIn() {
		return
	}

	id := c.Input().Get("id")
	cert, err := object.GetCert(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.ResponseOk(object.GetMaskedCert(cert))
}

func (c *ApiController) UpdateCert() {
	if c.RequireSignedIn() {
		return
	}

	id := c.Input().Get("id")

	var cert object.Cert
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &cert)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.Data["json"] = wrapActionResponse(object.UpdateCert(id, &cert))
	c.ServeJSON()
}

func (c *ApiController) AddCert() {
	if c.RequireSignedIn() {
		return
	}

	var cert object.Cert
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &cert)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.Data["json"] = wrapActionResponse(object.AddCert(&cert))
	c.ServeJSON()
}

func (c *ApiController) DeleteCert() {
	if c.RequireSignedIn() {
		return
	}

	var cert object.Cert
	err := json.Unmarshal(c.Ctx.Input.RequestBody, &cert)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	c.Data["json"] = wrapActionResponse(object.DeleteCert(&cert))
	c.ServeJSON()
}

func (c *ApiController) UpdateCertDomainExpire() {
	if c.RequireSignedIn() {
		return
	}

	id := c.Input().Get("id")
	cert, err := object.GetCert(id)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	domainExpireTime, err := object.GetCertDomainExpireTime(cert)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}

	if domainExpireTime == "" {
		c.ResponseError("Domain expire time is empty")
		return
	}
	cert.DomainExpireTime = domainExpireTime

	c.Data["json"] = wrapActionResponse(object.UpdateCert(id, cert))
	c.ServeJSON()
}
