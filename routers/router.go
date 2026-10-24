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

package routers

import (
	"github.com/beego/beego"
	"github.com/casbin/caswaf/controllers"
)

func init() {
	initAPI()
}

func initAPI() {
	ns :=
		beego.NewNamespace("/api",
			beego.NSInclude(
				&controllers.ApiController{},
			),
		)
	beego.AddNamespace(ns)

	beego.Router("/api/signin", &controllers.ApiController{}, "POST:Signin")
	beego.Router("/api/signout", &controllers.ApiController{}, "POST:Signout")
	beego.Router("/api/get-account", &controllers.ApiController{}, "GET:GetAccount")
	beego.Router("/api/get-providers", &controllers.ApiController{}, "GET:GetProviders")

	beego.Router("/api/get-global-nodes", &controllers.ApiController{}, "GET:GetGlobalNodes")
	beego.Router("/api/get-nodes", &controllers.ApiController{}, "GET:GetNodes")
	beego.Router("/api/get-node", &controllers.ApiController{}, "GET:GetNode")
	beego.Router("/api/update-node", &controllers.ApiController{}, "POST:UpdateNode")
	beego.Router("/api/add-node", &controllers.ApiController{}, "POST:AddNode")
	beego.Router("/api/delete-node", &controllers.ApiController{}, "POST:DeleteNode")

	beego.Router("/api/get-global-sites", &controllers.ApiController{}, "GET:GetGlobalSites")
	beego.Router("/api/get-sites", &controllers.ApiController{}, "GET:GetSites")
	beego.Router("/api/get-site", &controllers.ApiController{}, "GET:GetSite")
	beego.Router("/api/update-site", &controllers.ApiController{}, "POST:UpdateSite")
	beego.Router("/api/add-site", &controllers.ApiController{}, "POST:AddSite")
	beego.Router("/api/delete-site", &controllers.ApiController{}, "POST:DeleteSite")

	beego.Router("/api/get-global-certs", &controllers.ApiController{}, "GET:GetGlobalCerts")
	beego.Router("/api/get-certs", &controllers.ApiController{}, "GET:GetCerts")
	beego.Router("/api/get-cert", &controllers.ApiController{}, "GET:GetCert")
	beego.Router("/api/update-cert", &controllers.ApiController{}, "POST:UpdateCert")
	beego.Router("/api/add-cert", &controllers.ApiController{}, "POST:AddCert")
	beego.Router("/api/delete-cert", &controllers.ApiController{}, "POST:DeleteCert")
	beego.Router("/api/update-cert-domain-expire", &controllers.ApiController{}, "POST:UpdateCertDomainExpire")

	beego.Router("/api/get-applications", &controllers.ApiController{}, "GET:GetApplications")

	beego.Router("/api/get-records", &controllers.ApiController{}, "GET:GetRecords")
	beego.Router("/api/get-record", &controllers.ApiController{}, "GET:GetRecord")
	beego.Router("/api/delete-record", &controllers.ApiController{}, "POST:DeleteRecord")
	beego.Router("/api/update-record", &controllers.ApiController{}, "POST:UpdateRecord")
	beego.Router("/api/add-record", &controllers.ApiController{}, "POST:AddRecord")

	beego.Router("/api/get-metrics", &controllers.ApiController{}, "GET:GetMetrics")
	beego.Router("/api/get-metrics-over-time", &controllers.ApiController{}, "GET:GetMetricsOverTime")

	beego.Router("/api/get-setting", &controllers.ApiController{}, "GET:GetSetting")
	beego.Router("/api/update-setting", &controllers.ApiController{}, "POST:UpdateSetting")

	beego.Router("/api/get-rules", &controllers.ApiController{}, "GET:GetRules")
	beego.Router("/api/get-rule", &controllers.ApiController{}, "GET:GetRule")
	beego.Router("/api/add-rule", &controllers.ApiController{}, "POST:AddRule")
	beego.Router("/api/update-rule", &controllers.ApiController{}, "POST:UpdateRule")
	beego.Router("/api/delete-rule", &controllers.ApiController{}, "POST:DeleteRule")
}
