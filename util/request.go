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

package util

import (
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/casbin/caswaf/conf"
)

var (
	trustedProxies     []*net.IPNet
	trustedProxiesOnce sync.Once
)

func getTrustedProxies() []*net.IPNet {
	trustedProxiesOnce.Do(func() {
		value := conf.GetConfigString("trustedProxies")
		if value == "" {
			value = "127.0.0.0/8,::1/128,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,100.64.0.0/10,fc00::/7"
		}

		for _, cidr := range strings.Split(value, ",") {
			cidr = strings.TrimSpace(cidr)
			if cidr == "" {
				continue
			}
			if !strings.Contains(cidr, "/") {
				if strings.Contains(cidr, ":") {
					cidr += "/128"
				} else {
					cidr += "/32"
				}
			}

			_, ipNet, err := net.ParseCIDR(cidr)
			if err != nil {
				panic(err)
			}
			trustedProxies = append(trustedProxies, ipNet)
		}
	})
	return trustedProxies
}

func IsTrustedProxy(ip string) bool {
	netIp := net.ParseIP(ip)
	if netIp == nil {
		return false
	}

	for _, ipNet := range getTrustedProxies() {
		if ipNet.Contains(netIp) {
			return true
		}
	}
	return false
}

func GetRemoteIp(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func GetClientIp(r *http.Request) string {
	remoteIp := GetRemoteIp(r)
	if !IsTrustedProxy(remoteIp) {
		return remoteIp
	}

	forwarded := strings.Join(r.Header.Values("X-Forwarded-For"), ",")
	if forwarded != "" {
		clientIp := remoteIp
		ips := strings.Split(forwarded, ",")
		for i := len(ips) - 1; i >= 0; i-- {
			ip := strings.TrimSpace(ips[i])
			if net.ParseIP(ip) == nil {
				break
			}

			clientIp = ip
			if !IsTrustedProxy(ip) {
				break
			}
		}
		return clientIp
	}

	realIp := strings.TrimSpace(r.Header.Get("X-Real-IP"))
	if net.ParseIP(realIp) != nil {
		return realIp
	}

	return remoteIp
}
