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
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/domain"
)

// getAliyunDomainExpireTime returns the expiration time that Aliyun as the registrar has, which is
// what the Aliyun console shows. It returns a zero time if the domain is not registered in the
// Aliyun account of the given access key (e.g. the account only hosts its DNS).
func getAliyunDomainExpireTime(domainName string, accessKey string, accessSecret string) (time.Time, error) {
	client, err := domain.NewClientWithAccessKey(aliyunRegion, accessKey, accessSecret)
	if err != nil {
		return time.Time{}, err
	}

	request := domain.CreateQueryDomainByDomainNameRequest()
	request.Scheme = "https"
	request.DomainName = domainName

	response, err := client.QueryDomainByDomainName(request)
	if err != nil {
		return time.Time{}, err
	}

	// ExpirationDate is "2027-04-16 20:49:39" for a China site account but "Oct 21,2027 21:34:08"
	// for an international one, so use the timestamp instead.
	if response.ExpirationDateLong == 0 {
		return time.Time{}, nil
	}

	return time.UnixMilli(response.ExpirationDateLong), nil
}

// GetCertDomainExpireTime returns the domain expire time of the cert. For a cert that has an Aliyun
// access key, the Aliyun Domain API is asked first, since RDAP and whois of the registry may lag
// behind a renewal and Aliyun's RDAP blocks frequent queries with an anti-bot challenge.
func GetCertDomainExpireTime(cert *Cert) (string, error) {
	if cert.Provider == "Aliyun" && cert.AccessKey != "" && cert.AccessSecret != "" {
		domainName, err := getBaseDomain(cert.Name)
		if err != nil {
			return "", err
		}

		expireTime, err := getAliyunDomainExpireTime(domainName, cert.AccessKey, cert.AccessSecret)
		if err != nil {
			fmt.Printf("getAliyunDomainExpireTime() error for cert [%s]: %v\n", cert.GetId(), err)
		} else if !expireTime.IsZero() {
			return expireTime.Local().Format(time.RFC3339), nil
		}
	}

	return getDomainExpireTime(cert.Name)
}
