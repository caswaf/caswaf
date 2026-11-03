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
	"context"
	"fmt"
	"net/http"

	"github.com/casbin/caswaf/object"
)

type CompoundRule struct{}

type compoundDepthKey struct{}

// a compound rule that refers to itself, directly or through others, would otherwise recurse until the process crashes
const maxCompoundDepth = 8

// checkRule evaluates the expressions as an OR of AND groups, so "A and B or C and D" is "(A and B) or (C and D)"
func (r *CompoundRule) checkRule(expressions []*object.Expression, req *http.Request) (*RuleResult, error) {
	depth, _ := req.Context().Value(compoundDepthKey{}).(int)
	if depth >= maxCompoundDepth {
		return nil, fmt.Errorf("compound rules are nested more than %d levels deep, a compound rule may refer to itself", maxCompoundDepth)
	}
	req = req.WithContext(context.WithValue(req.Context(), compoundDepthKey{}, depth+1))

	anyGroup := false
	group := true
	for _, expression := range expressions {
		result, err := checkRules([]string{expression.Value}, req)
		if err != nil {
			return nil, err
		}
		isHit := result != nil

		switch expression.Operator {
		case "and", "begin":
			group = group && isHit
		case "or":
			anyGroup = anyGroup || group
			group = isHit
		default:
			return nil, fmt.Errorf("unknown operator: %s", expression.Operator)
		}
	}

	if anyGroup || group {
		return &RuleResult{}, nil
	}
	return nil, nil
}
