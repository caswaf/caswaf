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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/casbin/caswaf/proxy"
)

const rdapBootstrapUrl = "https://data.iana.org/rdap/dns.json"

var (
	rdapServers     map[string]string
	rdapServersLock sync.Mutex
)

type rdapEvent struct {
	EventAction string `json:"eventAction"`
	EventDate   string `json:"eventDate"`
}

type rdapLink struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}

type rdapDomain struct {
	Events []rdapEvent `json:"events"`
	Links  []rdapLink  `json:"links"`
}

func getRdapHttpClient() *http.Client {
	client := &http.Client{Timeout: 15 * time.Second}
	if proxy.ProxyHttpClient != nil {
		client.Transport = proxy.ProxyHttpClient.Transport
	}
	return client
}

func getRdapJson(client *http.Client, url string, v interface{}) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/rdap+json, application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("RDAP request [%s] returned status %d", url, resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(v)
}

func getRdapServer(client *http.Client, domainName string) (string, error) {
	rdapServersLock.Lock()
	defer rdapServersLock.Unlock()

	if rdapServers == nil {
		var bootstrap struct {
			Services [][][]string `json:"services"`
		}
		err := getRdapJson(client, rdapBootstrapUrl, &bootstrap)
		if err != nil {
			return "", err
		}

		servers := map[string]string{}
		for _, service := range bootstrap.Services {
			if len(service) != 2 || len(service[1]) == 0 {
				continue
			}
			for _, tld := range service[0] {
				servers[strings.ToLower(tld)] = service[1][0]
			}
		}
		rdapServers = servers
	}

	// abc.com.cn -> try "com.cn" first, then "cn"
	labels := strings.Split(strings.ToLower(domainName), ".")
	for i := 1; i < len(labels); i++ {
		if server, ok := rdapServers[strings.Join(labels[i:], ".")]; ok {
			return server, nil
		}
	}

	return "", fmt.Errorf("no RDAP server for domain: %s", domainName)
}

func getRdapEventTime(domain *rdapDomain, action string) (time.Time, bool) {
	for _, event := range domain.Events {
		if event.EventAction == action {
			t, err := time.Parse(time.RFC3339, event.EventDate)
			if err == nil {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

// getDomainExpireTimeFromRdap returns the registrar's and the registry's expiration time, a zero
// time for the one that is not available.
func getDomainExpireTimeFromRdap(domainName string) (time.Time, time.Time, error) {
	client := getRdapHttpClient()

	server, err := getRdapServer(client, domainName)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	var registryDomain rdapDomain
	err = getRdapJson(client, strings.TrimSuffix(server, "/")+"/domain/"+domainName, &registryDomain)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	registryTime, _ := getRdapEventTime(&registryDomain, "expiration")

	for _, link := range registryDomain.Links {
		if link.Rel != "related" || !strings.Contains(link.Href, "/domain/") {
			continue
		}

		var registrarDomain rdapDomain
		err = getRdapJson(client, link.Href, &registrarDomain)
		if err != nil {
			fmt.Printf("getDomainExpireTimeFromRdap(): registrar RDAP [%s] error: %v\n", link.Href, err)
			continue
		}

		for _, action := range []string{"registrar expiration", "expiration"} {
			if t, ok := getRdapEventTime(&registrarDomain, action); ok {
				return t, registryTime, nil
			}
		}

		// e.g. Aliyun answers 200 with an anti-bot challenge instead of the domain after a few queries
		fmt.Printf("getDomainExpireTimeFromRdap(): registrar RDAP [%s] has no expiration event\n", link.Href)
	}

	return time.Time{}, registryTime, nil
}
