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
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/casbin/caswaf/run"
	"github.com/casbin/caswaf/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"golang.org/x/net/publicsuffix"
)

func resolveDomainToIp(domain string) string {
	ips, err := net.LookupIP(domain)
	if err != nil {
		if strings.Contains(err.Error(), "no such host") {
			return "(empty)"
		}

		fmt.Printf("resolveDomainToIp() error: %s\n", err.Error())
		return err.Error()
	}

	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String()
		}
	}
	return "(empty)"
}

func getBaseDomain(domain string) (string, error) {
	// abc.com -> abc.com
	// abc.com.it -> abc.com.it
	// subdomain.abc.io -> abc.io
	// subdomain.abc.org.us -> abc.org.us
	return publicsuffix.EffectiveTLDPlusOne(domain)
}

func pingUrl(url string) (bool, string) {
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get(url)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		return true, ""
	}
	return false, fmt.Sprintf("Status: %s", resp.Status)
}

type VersionInfo struct {
	Version      string `json:"version"`
	CommitId     string `json:"commitId"`
	CommitOffset int    `json:"commitOffset"`
}

func getVersionInfo(path string) (*VersionInfo, error) {
	res := &VersionInfo{
		Version:      "",
		CommitId:     "",
		CommitOffset: -1,
	}

	r, err := git.PlainOpen(path)
	if err != nil {
		return res, err
	}
	ref, err := r.Head()
	if err != nil {
		return res, err
	}
	tags, err := r.Tags()
	if err != nil {
		return res, err
	}
	tagMap := make(map[plumbing.Hash]string)
	err = tags.ForEach(func(t *plumbing.Reference) error {
		// This technique should work for both lightweight and annotated tags.
		revHash, err := r.ResolveRevision(plumbing.Revision(t.Name()))
		if err != nil {
			return err
		}
		tagMap[*revHash] = t.Name().Short()
		return nil
	})
	if err != nil {
		return res, err
	}

	cIter, err := r.Log(&git.LogOptions{From: ref.Hash()})
	if err != nil {
		return res, err
	}

	commitOffset := 0
	version := ""
	// iterates over the commits
	err = cIter.ForEach(func(c *object.Commit) error {
		tag, ok := tagMap[c.Hash]
		if ok {
			if version == "" {
				version = tag
			}
		}
		if version == "" {
			commitOffset++
		}
		return nil
	})
	if err != nil {
		return res, err
	}

	res = &VersionInfo{
		Version:      version,
		CommitId:     ref.Hash().String(),
		CommitOffset: commitOffset,
	}
	return res, nil
}

func getSiteVersion(siteName string) (string, error) {
	path := run.GetRepoPath(siteName)
	versionInfo, err := getVersionInfo(path)
	if err != nil {
		return "", err
	}

	res := util.StructToJsonNoIndent(versionInfo)
	return res, nil
}

func getNodeNameFromTag(s string) string {
	tokens := strings.SplitN(s, "-", 2)
	if len(tokens) >= 2 {
		return tokens[1]
	} else {
		return s
	}
}

func getCertMap() (map[string]*Cert, error) {
	certs, err := GetGlobalCerts()
	if err != nil {
		return nil, err
	}

	res := map[string]*Cert{}
	for _, cert := range certs {
		res[cert.Name] = cert
	}
	return res, nil
}

func GetCertByDomain(domain string) (*Cert, error) {
	if domain == "" {
		return nil, fmt.Errorf("GetCertByDomain() error: domain should not be empty")
	}

	cert, ok := certMap[domain]
	if ok {
		return cert, nil
	}

	baseDomain, err := getBaseDomain(domain)
	if err != nil {
		return nil, err
	}

	cert, ok = certMap[baseDomain]
	if ok {
		return cert, nil
	}

	return nil, nil
}
