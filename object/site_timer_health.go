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

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

var healthCheckTryTimesMap = map[string]int{}

func healthCheck(site *Site, domain string) error {
	var isHealth bool
	var pingResponse string
	urlHttps := "https://" + domain
	urlHttp := "http://" + domain
	switch site.SslMode {
	case "HTTPS Only":
		isHealth, pingResponse = pingUrl(urlHttps)
	case "HTTP":
		isHealth, pingResponse = pingUrl(urlHttp)
	case "HTTPS and HTTP":
		isHttpsHealth, httpsPingResponse := pingUrl(urlHttps)
		isHttpHealth, httpPingResponse := pingUrl(urlHttp)
		isHealth = isHttpsHealth || isHttpHealth
		pingResponse = httpsPingResponse + httpPingResponse
	}

	if isHealth {
		healthCheckTryTimesMap[domain] = GetSiteByDomain(domain).AlertTryTimes
		return nil
	}

	healthCheckTryTimesMap[domain]--
	if healthCheckTryTimesMap[domain] != 0 {
		return nil
	}

	pingResponse = fmt.Sprintf("CasWAF health check failed for domain %s, %s", domain, pingResponse)
	user, err := casdoorsdk.GetUser(site.Owner)
	if err != nil {
		return err
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}
	for _, provider := range site.AlertProviders {
		if strings.HasPrefix(provider, "Email/") {
			err := casdoorsdk.SendEmailByProvider("CasWAF HealthCheck Check Alert", pingResponse, "CasWAF", provider[6:], user.Email)
			if err != nil {
				fmt.Println(err)
			}
		}
		if strings.HasPrefix(provider, "SMS/") {
			err := casdoorsdk.SendSmsByProvider(pingResponse, provider[4:], user.Phone)
			if err != nil {
				fmt.Println(err)
			}
		}
	}
	return nil
}

func startHealthCheckLoop() {
	for _, domain := range healthCheckNeededDomains {
		domain := domain
		if _, ok := healthCheckTryTimesMap[domain]; ok {
			continue
		}
		healthCheckTryTimesMap[domain] = GetSiteByDomain(domain).AlertTryTimes
		go func() {
			for {
				site := GetSiteByDomain(domain)
				if shouldStopHealthCheck(site) {
					delete(healthCheckTryTimesMap, domain)
					return
				}

				err := healthCheck(site, domain)
				if err != nil {
					fmt.Println(err)
				}
				time.Sleep(time.Duration(site.AlertInterval) * time.Second)
			}
		}()
	}
}

func shouldStopHealthCheck(site *Site) bool {
	return site == nil || !site.EnableAlert || site.Domain == "" || site.Status == "Inactive" || site.Status == "Stopped" || site.Status == "Deleting"
}
