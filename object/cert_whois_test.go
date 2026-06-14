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

//go:build !skipCi
// +build !skipCi

package object

import (
	"fmt"
	"testing"
)

func TestUpdateDomainExpireTime(t *testing.T) {
	InitConfig()

	certs, err := GetCerts("admin")
	if err != nil {
		panic(err)
	}

	for i, cert := range certs {
		if cert.Name == "casbin.io" {
			continue
		}

		certExpireTime, err := getDomainExpireTime(cert.Name)
		if err != nil {
			panic(err)
		}

		if cert.DomainExpireTime == certExpireTime {
			continue
		}

		cert.DomainExpireTime = certExpireTime

		res, err := UpdateCert(cert.GetId(), cert)
		if err != nil {
			panic(err)
		}

		fmt.Printf("[%d/%d] Refreshed cert [%s]'s domain expire time: [%s], res = %v\n", i+1, len(certs), cert.Name, cert.DomainExpireTime, res)
	}
}
