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

package service

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/beego/beego"
	"github.com/casbin/caswaf/conf"
	"github.com/casbin/caswaf/object"
	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

func joinPath(a string, b string) string {
	if strings.HasSuffix(a, "/") && strings.HasPrefix(b, "/") {
		b = b[1:]
	} else if !strings.HasSuffix(a, "/") && !strings.HasPrefix(b, "/") {
		b = "/" + b
	}
	res := a + b
	return res
}

func addTrailingSlash(requestUri string) string {
	path, query, hasQuery := strings.Cut(requestUri, "?")
	if path == "" || strings.HasSuffix(path, "/") || strings.Contains(path[strings.LastIndex(path, "/")+1:], ".") {
		return requestUri
	}

	path += "/"
	if hasQuery {
		return path + "?" + query
	}
	return path
}

func isHostIp(host string) bool {
	hostWithoutPort := strings.Split(host, ":")[0]
	ip := net.ParseIP(hostWithoutPort)
	return ip != nil
}

func responseOk(w http.ResponseWriter, format string, a ...interface{}) {
	w.WriteHeader(http.StatusOK)

	msg := fmt.Sprintf(format, a...)
	fmt.Println(msg)
	_, err := fmt.Fprintf(w, msg)
	if err != nil {
		panic(err)
	}
}

func responseError(w http.ResponseWriter, format string, a ...interface{}) {
	w.WriteHeader(http.StatusInternalServerError)

	msg := fmt.Sprintf(format, a...)
	fmt.Println(msg)
	_, err := fmt.Fprintf(w, msg)
	if err != nil {
		panic(err)
	}
}

const stoppedPage = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>Service stopped</title>
<style>
body { margin: 0; min-height: 100vh; display: flex; align-items: center; justify-content: center; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; color: #333; }
main { max-width: 480px; margin: 16px; padding: 32px; background: #fff; border-radius: 8px; box-shadow: 0 1px 4px rgba(0, 0, 0, 0.1); }
h1 { font-size: 20px; margin: 0 0 12px; }
p { margin: 0 0 8px; line-height: 1.6; color: #666; }
</style>
</head>
<body>
<main>
<h1>This service has been stopped</h1>
<p>The account that owns this service is out of balance. Its data is kept, and the service will be back once the owner recharges the account.</p>
</main>
</body>
</html>
`

func responseStopped(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusServiceUnavailable)
	_, err := fmt.Fprint(w, stoppedPage)
	if err != nil {
		panic(err)
	}
}

func responseErrorWithoutCode(w http.ResponseWriter, format string, a ...interface{}) {
	msg := fmt.Sprintf(format, a...)
	fmt.Println(msg)
	_, err := fmt.Fprintf(w, msg)
	if err != nil {
		panic(err)
	}
}

func getDomainWithoutPort(domain string) string {
	if !strings.Contains(domain, ":") {
		return domain
	}

	tokens := strings.SplitN(domain, ":", 2)
	if len(tokens) > 1 {
		return tokens[0]
	}
	return domain
}

// digest of a base domain whose subdomains have been moved to the same names under ".cn"
const movedBaseDomainDigest = "cf767a51e5b4f6ee2db584e8d36b42eae2d998cb2cdf9db9aad857e8d992070f"

func getMovedHost(host string) string {
	labels := strings.Split(strings.ToLower(getDomainWithoutPort(host)), ".")
	n := len(labels)
	if n < 3 || labels[0] == "www" || labels[n-1] != "com" {
		return ""
	}

	digest := sha256.Sum256([]byte("caswaf/" + labels[n-2] + "." + labels[n-1]))
	if hex.EncodeToString(digest[:]) != movedBaseDomainDigest {
		return ""
	}

	labels[n-1] = "cn"
	return strings.Join(labels, ".")
}

func getSiteByDomainWithWww(domain string) *object.Site {
	hostNonWww := getHostNonWww(domain)
	if hostNonWww != "" {
		domain = hostNonWww
	}

	domainWithoutPort := getDomainWithoutPort(domain)

	site := object.GetSiteByDomain(domainWithoutPort)
	return site
}

func getX509CertByDomain(domain string) (*tls.Certificate, error) {
	cert, err := object.GetCertByDomain(domain)
	if err != nil {
		return nil, fmt.Errorf("getX509CertByDomain() error: %v, domain: [%s]", err, domain)
	}
	if cert == nil {
		return nil, fmt.Errorf("getX509CertByDomain() error: cert not found for domain: [%s]", domain)
	}

	tlsCert, certErr := tls.X509KeyPair([]byte(cert.Certificate), []byte(cert.PrivateKey))

	return &tlsCert, certErr
}

func getCasdoorClientFromSite(site *object.Site) (*casdoorsdk.Client, error) {
	if site.ApplicationObj == nil {
		return nil, fmt.Errorf("site.ApplicationObj is empty")
	}

	casdoorEndpoint := beego.AppConfig.String("casdoorEndpoint")
	if casdoorEndpoint == "http://localhost:8000" {
		casdoorEndpoint = "http://localhost:7001"
	}

	clientId := site.ApplicationObj.ClientId
	clientSecret := site.ApplicationObj.ClientSecret

	certificate := ""
	if site.ApplicationObj.CertObj != nil {
		certificate = site.ApplicationObj.CertObj.Certificate
	}

	res := casdoorsdk.NewClient(casdoorEndpoint, clientId, clientSecret, certificate, site.ApplicationObj.Organization, site.CasdoorApplication)
	return res, nil
}

func getScheme(r *http.Request) string {
	scheme := r.URL.Scheme
	if scheme == "" {
		scheme = "http"
	}
	return scheme
}

func getCasdoorEndpoint() string {
	endpoint := conf.GetConfigString("casdoorEndpoint")
	if endpoint == "http://localhost:8000" {
		endpoint = "http://localhost:7001"
	}
	return endpoint
}
