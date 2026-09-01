// Copyright 2021 The casbin Authors. All Rights Reserved.
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

//go:build !skipCi
// +build !skipCi

package certificate

import (
	"testing"

	"github.com/beego/beego"
	"github.com/casbin/caswaf/proxy"
	"github.com/casbin/caswaf/util"
)

func TestGetClient(t *testing.T) {
	err := beego.LoadAppConfig("ini", "../conf/app.conf")
	if err != nil {
		panic(err)
	}

	proxy.InitHttpClient()

	eccKey := util.ReadStringFromPath("acme_account.key")
	println(eccKey)

	client := GetAcmeClient("acme2@casbin.org", eccKey, false)
	pem, key, err := ObtainCertificateAli(client, "casbin.com", accessKeyId, accessKeySecret)
	if err != nil {
		panic(err)
	}
	println(pem)
	println()
	println(key)
}
