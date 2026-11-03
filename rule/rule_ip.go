// Copyright 2024 The casbin Authors. All Rights Reserved.
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

package rule

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/casbin/caswaf/ip"
	"github.com/casbin/caswaf/object"
	"github.com/casbin/caswaf/util"
)

type IpRule struct{}

func (r *IpRule) checkRule(expressions []*object.Expression, req *http.Request) (*RuleResult, error) {
	clientIp := util.GetClientIp(req)
	netIp, err := parseIp(clientIp)
	if err != nil {
		return nil, err
	}
	for _, expression := range expressions {
		reason := fmt.Sprintf("expression matched: \"%s %s %s\"", clientIp, expression.Operator, expression.Value)

		// Handle "is abroad" operator
		if expression.Operator == "is abroad" {
			if ip.IsAbroadIp(clientIp) {
				return &RuleResult{Reason: reason}, nil
			}
			continue
		}

		isIn := false
		ips := strings.Split(expression.Value, ",")
		for _, ipStr := range ips {
			ipStr = strings.TrimSpace(ipStr)
			if strings.Contains(ipStr, "/") {
				_, ipNet, err := net.ParseCIDR(ipStr)
				if err != nil {
					return nil, err
				}
				if ipNet.Contains(netIp) {
					isIn = true
				}
			} else if strings.ContainsAny(ipStr, ".:") {
				if ip := net.ParseIP(ipStr); ip != nil && ip.Equal(netIp) {
					isIn = true
				}
			} else {
				return nil, fmt.Errorf("unknown IP or CIDR format: %s", ipStr)
			}
		}

		// "is not in" matches a client outside every address and range of the list, not outside any one of them
		switch expression.Operator {
		case "is in":
			if isIn {
				return &RuleResult{Reason: reason}, nil
			}
		case "is not in":
			if !isIn {
				return &RuleResult{Reason: reason}, nil
			}
		default:
			return nil, fmt.Errorf("unknown operator: %s", expression.Operator)
		}
	}
	return nil, nil
}

func parseIp(ipStr string) (net.IP, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("unknown IP or CIDR format: %s", ipStr)
	}
	return ip, nil
}
