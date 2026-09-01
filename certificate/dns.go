// Copyright 2021 The casbin Authors. All Rights Reserved.
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

package certificate

import (
	"fmt"
	"strings"
	"time"

	"github.com/casbin/lego/v4/certificate"
	"github.com/casbin/lego/v4/challenge"
	"github.com/casbin/lego/v4/challenge/dns01"
	"github.com/casbin/lego/v4/cmd"
	"github.com/casbin/lego/v4/lego"
	"github.com/casbin/lego/v4/log"
	"github.com/casbin/lego/v4/providers/dns/alidns"
	"github.com/casbin/lego/v4/providers/dns/godaddy"
)

const (
	obtainRetryCount    = 3
	obtainRetryInterval = 30 * time.Second
	dnsSettleWait       = 20 * time.Second
	dnsQueryTimeout     = 20 * time.Second
)

// Choose local DNS service providers to increase the authentication speed
var recursiveNameservers = []string{"223.5.5.5:53", "119.29.29.29:53"}

type AliConf struct {
	Domains       []string // The domain names for which you want to apply for a certificate
	AccessKey     string   // Aliyun account's AccessKey, if this is not empty, Secret is required.
	Secret        string
	RAMRole       string // Use Ramrole to control aliyun account
	SecurityToken string // Optional
	Path          string // The path to store cert file
	Timeout       int    // Maximum waiting time for certificate application, in minutes
}

type GodaddyConf struct {
	Domains   []string // The domain names for which you want to apply for a certificate
	APIKey    string   // GoDaddy account's API Key
	APISecret string
	Path      string // The path to store cert file
	Timeout   int    // Maximum waiting time for certificate application, in minutes
}

// getChallengeOptions requires the TXT record to be visible on every authoritative name
// server of the zone, and waits an extra settle time afterwards, otherwise the ACME server
// may query the record before the DNS provider finishes syncing it, which fails randomly.
func getChallengeOptions() []dns01.ChallengeOption {
	return []dns01.ChallengeOption{
		dns01.AddRecursiveNameservers(dns01.ParseNameservers(recursiveNameservers)),
		dns01.AddDNSTimeout(dnsQueryTimeout),
		dns01.WrapPreCheck(func(domain string, fqdn string, value string, check dns01.PreCheckFunc) (bool, error) {
			ok, err := check(fqdn, value)
			if err != nil || !ok {
				return ok, err
			}

			log.Infof("[%s] acme: DNS record propagated, waiting %s before validation", domain, dnsSettleWait)
			time.Sleep(dnsSettleWait)
			return true, nil
		}),
	}
}

// isRetriableError reports whether the failure is transient, the most common one is the
// ACME server timing out while looking up the TXT record.
func isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	keywords := []string{
		"dns problem",
		"timed out",
		"timeout",
		"connection reset",
		"connection refused",
		"unexpected eof",
		"servererror",
		"internal server error",
		"bad gateway",
		"service unavailable",
	}
	for _, keyword := range keywords {
		if strings.Contains(message, keyword) {
			return true
		}
	}

	return false
}

// obtainCert verifies the domain ownership via the given DNS provider, then obtains a
// certificate, retrying the transient failures.
func obtainCert(client *lego.Client, dnsProvider challenge.Provider, domains []string) (string, string, error) {
	err := client.Challenge.SetDNS01Provider(dnsProvider, getChallengeOptions()...)
	if err != nil {
		return "", "", err
	}

	request := certificate.ObtainRequest{
		Domains: domains,
		Bundle:  true,
	}

	for i := 0; i < obtainRetryCount; i++ {
		if i > 0 {
			log.Infof("[%s] acme: Retrying to obtain the certificate, attempt [%d/%d]", strings.Join(domains, ", "), i+1, obtainRetryCount)
			time.Sleep(obtainRetryInterval)
		}

		dns01.ClearFqdnCache()

		var cert *certificate.Resource
		cert, err = client.Certificate.Obtain(request)
		if err == nil {
			return string(cert.Certificate), string(cert.PrivateKey), nil
		}

		if !isRetriableError(err) {
			return "", "", err
		}

		log.Infof("[%s] acme: Failed to obtain the certificate: %v", strings.Join(domains, ", "), err)
	}

	return "", "", fmt.Errorf("failed to obtain the certificate for [%s] after %d attempts: %w", strings.Join(domains, ", "), obtainRetryCount, err)
}

// getAliCert Verify domain ownership, then obtain a certificate, and finally store it locally.
// Need to pass in an AliConf struct, some parameters are required, other parameters can be left blank
func getAliCert(client *lego.Client, conf AliConf) (string, string, error) {
	if conf.Timeout <= 0 {
		conf.Timeout = 3
	}

	config := alidns.NewDefaultConfig()
	config.PropagationTimeout = time.Duration(conf.Timeout) * time.Minute
	config.APIKey = conf.AccessKey
	config.SecretKey = conf.Secret
	config.RAMRole = conf.RAMRole
	config.SecurityToken = conf.SecurityToken

	dnsProvider, err := alidns.NewDNSProvider(config)
	if err != nil {
		return "", "", err
	}

	return obtainCert(client, dnsProvider, conf.Domains)
}

func getGoDaddyCert(client *lego.Client, conf GodaddyConf) (string, string, error) {
	if conf.Timeout <= 0 {
		conf.Timeout = 3
	}

	config := godaddy.NewDefaultConfig()
	config.PropagationTimeout = time.Duration(conf.Timeout) * time.Minute
	config.PollingInterval = time.Duration(conf.Timeout) * time.Minute / 9
	config.APIKey = conf.APIKey
	config.APISecret = conf.APISecret

	dnsProvider, err := godaddy.NewDNSProvider(config)
	if err != nil {
		return "", "", err
	}

	return obtainCert(client, dnsProvider, conf.Domains)
}

func ObtainCertificateAli(client *lego.Client, domain string, accessKey string, accessSecret string) (string, string, error) {
	conf := AliConf{
		Domains:       []string{fmt.Sprintf("*.%s", domain), domain},
		AccessKey:     accessKey,
		Secret:        accessSecret,
		RAMRole:       "",
		SecurityToken: "",
		Path:          "",
		Timeout:       3,
	}
	return getAliCert(client, conf)
}

func ObtainCertificateGoDaddy(client *lego.Client, domain string, accessKey string, accessSecret string) (string, string, error) {
	conf := GodaddyConf{
		Domains:   []string{fmt.Sprintf("*.%s", domain), domain},
		APIKey:    accessKey,
		APISecret: accessSecret,
		Path:      "",
		Timeout:   3,
	}
	return getGoDaddyCert(client, conf)
}

func SaveCert(path, filename string, cert *certificate.Resource) {
	// Store the certificate file locally
	certsStorage := cmd.NewCertificatesStorageLib(path, filename, true)
	certsStorage.CreateRootFolder()
	certsStorage.SaveResource(cert)
}
