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

package controllers

import (
	"errors"
	"time"

	"github.com/casbin/caswaf/object"
	"github.com/casbin/caswaf/util"
)

func (c *ApiController) GetMetricsOverTime() {
	if c.RequireAdmin() {
		return
	}
	rangeType := c.Input().Get("rangeType")
	count := util.ParseInt(c.Input().Get("count"))
	granularity := c.Input().Get("granularity")
	timeType := granularity2TimeType(granularity)
	startTime := time.Now().Add(time.Duration(-count) * rangeType2Duration(rangeType))
	metrics, err := object.GetMetricsOverTime(startTime, timeType)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	var total int64
	for _, metric := range *metrics {
		total += metric.Count
	}
	c.ResponseOk(metrics, total)
}

func granularity2TimeType(rangeType string) string {
	switch rangeType {
	case "hour":
		return "hour"
	case "day":
		return "day"
	case "week":
		return "day"
	case "month":
		return "month"
	case "year":
		return "month"
	default:
		return "month"
	}
}

func (c *ApiController) GetMetrics() {
	if c.RequireAdmin() {
		return
	}

	dtoType := c.Input().Get("type")
	dataType, err := type2DataType(dtoType)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	rangeType := c.Input().Get("rangeType")
	count := util.ParseInt(c.Input().Get("count"))
	top, err := util.ParseIntWithError(c.Input().Get("top"))
	// if top is not set or invalid, set it to the maximum value
	if err != nil || top <= 0 {
		top = int(^uint(0) >> 1)
	}
	startTime := time.Now().Add(time.Duration(-count) * rangeType2Duration(rangeType))
	metrics, err := object.GetMetrics(dataType, startTime, top)
	if err != nil {
		c.ResponseError(err.Error())
		return
	}
	var total int64
	for _, metric := range *metrics {
		total += metric.Count
	}
	c.ResponseOk(metrics, total)
}

func rangeType2Duration(rangeType string) time.Duration {
	switch rangeType {
	case "hour":
		return time.Hour
	case "day":
		return 24 * time.Hour
	case "week":
		return 7 * 24 * time.Hour
	case "month":
		return 30 * 24 * time.Hour
	case "year":
		return 365 * 24 * time.Hour
	default:
		return time.Hour
	}
}

func type2DataType(dataType string) (string, error) {
	switch dataType {
	case "site":
		return "host", nil
	case "path":
		return "path", nil
	case "ip":
		return "client_ip", nil
	case "userAgent":
		return "user_agent", nil
	default:
		return "", errors.New("invalid data type")
	}
}
