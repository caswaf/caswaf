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
	"strings"
	"time"

	"github.com/casbin/caswaf/proxy"
	"github.com/likexian/whois"
	whoisparser "github.com/likexian/whois-parser"
)

// The registry only learns about a renewal when the registrar pushes it, which some registrars
// (e.g. Aliyun) delay until the domain is about to expire, so the registrar's expiration time is
// preferred: registrar RDAP, then registrar whois, then the registry's own record.
func getDomainExpireTime(domainName string) (string, error) {
	domainName, err := getBaseDomain(domainName)
	if err != nil {
		return "", err
	}

	rdapRegistrarTime, rdapRegistryTime, err := getDomainExpireTimeFromRdap(domainName)
	if err != nil {
		fmt.Printf("getDomainExpireTimeFromRdap() error: %v\n", err)
	}
	if !rdapRegistrarTime.IsZero() {
		return rdapRegistrarTime.Local().Format(time.RFC3339), nil
	}

	whoisRegistrarTime, whoisRegistryTime, err := getDomainExpireTimeFromWhois(domainName)
	if err != nil {
		fmt.Printf("getDomainExpireTimeFromWhois() error: %v\n", err)
	}
	if !whoisRegistrarTime.IsZero() {
		return whoisRegistrarTime.Local().Format(time.RFC3339), nil
	}

	if !rdapRegistryTime.IsZero() {
		return rdapRegistryTime.Local().Format(time.RFC3339), nil
	}
	if err != nil {
		return "", err
	}
	return whoisRegistryTime.Local().Format(time.RFC3339), nil
}

// getDomainExpireTimeFromWhois returns the registrar's and the registry's expiration time. The whois
// client follows the registry's "Registrar WHOIS Server" referral and appends the registrar's answer.
func getDomainExpireTimeFromWhois(domainName string) (time.Time, time.Time, error) {
	server := ""
	if strings.HasSuffix(domainName, ".com") || strings.HasSuffix(domainName, ".net") {
		server = "whois.verisign-grs.com"
	} else if strings.HasSuffix(domainName, ".org") {
		server = "whois.pir.org"
	} else if strings.HasSuffix(domainName, ".io") {
		server = "whois.nic.io"
	} else if strings.HasSuffix(domainName, ".co") {
		server = "whois.nic.co"
	} else if strings.HasSuffix(domainName, ".cn") {
		server = "whois.cnnic.cn"
	} else if strings.HasSuffix(domainName, ".run") {
		server = "whois.nic.run"
	} else {
		server = "grs-whois.hichina.com" // com, net, cc, tv
		//return "", fmt.Errorf("unsupported suffix for domain: %s", domainName)
	}

	client := whois.NewClient()
	if server != "whois.cnnic.cn" && server != "grs-whois.hichina.com" {
		dialer := proxy.GetProxyDialer()
		if dialer != nil {
			client.SetDialer(dialer)
		}
	}

	data, err := client.Whois(domainName, server)
	if err != nil {
		if !strings.HasSuffix(domainName, ".run") || data == "" {
			return time.Time{}, time.Time{}, err
		}
	}

	registrarTime := getWhoisRegistrarExpireTime(data)

	whoisInfo, err := whoisparser.Parse(data)
	if err != nil {
		return registrarTime, time.Time{}, err
	}

	if whoisInfo.Domain == nil || whoisInfo.Domain.ExpirationDateInTime == nil {
		return registrarTime, time.Time{}, fmt.Errorf("no expiration date in whois for domain: %s", domainName)
	}

	return registrarTime, *whoisInfo.Domain.ExpirationDateInTime, nil
}

func getWhoisRegistrarExpireTime(data string) time.Time {
	const prefix = "Registrar Registration Expiration Date:"
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}

		value := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
			t, err := time.Parse(layout, value)
			if err == nil {
				return t
			}
		}
	}
	return time.Time{}
}
