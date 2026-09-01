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
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/cas"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/cdn"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/dcdn"
	"github.com/aliyun/alibaba-cloud-sdk-go/services/vod"
)

const (
	aliyunRegion    = "cn-hangzhou"
	aliyunVodRegion = "cn-shanghai"

	aliyunPageSize = 50
)

// aliyunDomain is a domain of an Aliyun product whose HTTPS config needs the new cert.
type aliyunDomain struct {
	product     string // "VOD", "CDN" or "DCDN"
	name        string
	sslProtocol string
	cert        *Cert
}

// aliyunAccount is an Aliyun account found in the cert table. Many certs usually share the same
// account, so the account is visited only once instead of once per cert.
type aliyunAccount struct {
	accessKey    string
	accessSecret string
	account      string // the Aliyun user name, used for logging only
	certName     string // the cert that the credential comes from, used for logging only

	casClient  *cas.Client
	vodClient  *vod.Client
	cdnClient  *cdn.Client
	dcdnClient *dcdn.Client
}

// DeployCertsToAliyun uploads the certs owned by "admin" to the Aliyun accounts stored in the cert
// table, then binds them to the HTTPS config of the VOD, CDN and DCDN domains of those accounts.
// A cert is only uploaded when at least one of those domains actually uses it.
func DeployCertsToAliyun() error {
	certs, err := GetCerts("admin")
	if err != nil {
		return err
	}

	// A cert can be issued by any provider, e.g. a GoDaddy cert whose domain is served by Aliyun
	// CDN, so all the certs are candidates, no matter which provider issued them.
	usableCerts := []*Cert{}
	for _, cert := range certs {
		if cert.Certificate == "" || cert.PrivateKey == "" {
			continue
		}

		usableCerts = append(usableCerts, cert)
	}

	accounts := getAliyunAccounts(certs)
	if len(accounts) == 0 {
		return fmt.Errorf("DeployCertsToAliyun() error: no Aliyun account is found in the cert table")
	}

	fmt.Printf("Deploying %d cert(s) to %d Aliyun account(s)\n", len(usableCerts), len(accounts))

	failures := []string{}
	for i, account := range accounts {
		fmt.Printf("[%d/%d] Aliyun account: [%s], from cert: [%s]\n", i+1, len(accounts), account.getId(), account.certName)

		err = account.initClients()
		if err != nil {
			fmt.Printf("  Failed to create the Aliyun clients, err = %v\n", err)
			failures = append(failures, fmt.Sprintf("account [%s]: %v", account.getId(), err))
			continue
		}

		for _, failure := range account.deployCerts(usableCerts) {
			failures = append(failures, fmt.Sprintf("account [%s]: %s", account.getId(), failure))
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("DeployCertsToAliyun() error, %d step(s) failed:\n%s", len(failures), strings.Join(failures, "\n"))
	}

	fmt.Printf("All the certs are deployed to %d Aliyun account(s)\n", len(accounts))
	return nil
}

// getAliyunAccounts collects the distinct Aliyun credentials stored in the certs.
func getAliyunAccounts(certs []*Cert) []*aliyunAccount {
	accounts := []*aliyunAccount{}
	visitedMap := map[string]bool{}
	for _, cert := range certs {
		if cert.Provider != "Aliyun" || cert.AccessKey == "" || cert.AccessSecret == "" {
			continue
		}

		id := fmt.Sprintf("%s/%s", cert.AccessKey, cert.AccessSecret)
		if visitedMap[id] {
			continue
		}
		visitedMap[id] = true

		accounts = append(accounts, &aliyunAccount{
			accessKey:    cert.AccessKey,
			accessSecret: cert.AccessSecret,
			account:      cert.Account,
			certName:     cert.Name,
		})
	}

	return accounts
}

// getId returns the Aliyun user name of the account, it falls back to the masked access key for the
// certs that have no account filled in.
func (account *aliyunAccount) getId() string {
	if account.account != "" {
		return account.account
	}

	if len(account.accessKey) <= 8 {
		return account.accessKey
	}

	return fmt.Sprintf("%s***%s", account.accessKey[:4], account.accessKey[len(account.accessKey)-4:])
}

func (account *aliyunAccount) initClients() error {
	var err error
	account.casClient, err = cas.NewClientWithAccessKey(aliyunRegion, account.accessKey, account.accessSecret)
	if err != nil {
		return err
	}

	account.vodClient, err = vod.NewClientWithAccessKey(aliyunVodRegion, account.accessKey, account.accessSecret)
	if err != nil {
		return err
	}

	account.cdnClient, err = cdn.NewClientWithAccessKey(aliyunRegion, account.accessKey, account.accessSecret)
	if err != nil {
		return err
	}

	account.dcdnClient, err = dcdn.NewClientWithAccessKey(aliyunRegion, account.accessKey, account.accessSecret)
	if err != nil {
		return err
	}

	return nil
}

// deployCerts runs the whole flow for one account and returns the failed steps, so that a single
// broken domain does not stop the other ones.
func (account *aliyunAccount) deployCerts(certs []*Cert) []string {
	failures := []string{}

	domains := []*aliyunDomain{}
	listers := []struct {
		product string
		getFunc func() ([]*aliyunDomain, error)
	}{
		{"VOD", account.getVodDomains},
		{"CDN", account.getCdnDomains},
		{"DCDN", account.getDcdnDomains},
	}
	for _, lister := range listers {
		productDomains, err := lister.getFunc()
		if err != nil {
			if isServiceNotOpenError(err) {
				fmt.Printf("  [%s] The service is not activated for this account, skipped\n", lister.product)
				continue
			}

			fmt.Printf("  [%s] Failed to list the domains, err = %v\n", lister.product, err)
			failures = append(failures, fmt.Sprintf("[%s] failed to list the domains: %v", lister.product, err))
			continue
		}

		fmt.Printf("  [%s] Listed %d domain(s)\n", lister.product, len(productDomains))
		domains = append(domains, productDomains...)
	}

	// Only the domains that have HTTPS on and that one of our certs covers are updated. The certs
	// that no domain uses are not uploaded at all.
	targets := []*aliyunDomain{}
	usedCerts := []*Cert{}
	usedCertMap := map[string]bool{}
	for _, domain := range domains {
		if !strings.EqualFold(domain.sslProtocol, "on") {
			fmt.Printf("  [%s] Skipped domain: [%s], its HTTPS is not enabled\n", domain.product, domain.name)
			continue
		}

		domain.cert = matchCert(domain.name, certs)
		if domain.cert == nil {
			fmt.Printf("  [%s] Skipped domain: [%s], no cert covers it\n", domain.product, domain.name)
			continue
		}

		targets = append(targets, domain)
		if !usedCertMap[domain.cert.GetId()] {
			usedCertMap[domain.cert.GetId()] = true
			usedCerts = append(usedCerts, domain.cert)
		}
	}

	if len(targets) == 0 {
		fmt.Printf("  No domain needs to be updated, nothing is uploaded\n")
		return failures
	}

	casCerts, err := account.getCasCerts()
	if err != nil {
		fmt.Printf("  [CAS] Failed to list the uploaded certs, err = %v\n", err)
		return append(failures, fmt.Sprintf("[CAS] failed to list the uploaded certs: %v", err))
	}

	casNameMap := map[string]string{}
	for i, cert := range usedCerts {
		casName, err2 := account.uploadCert(casCerts, cert)
		if err2 != nil {
			fmt.Printf("  [%d/%d] [CAS] Failed to upload cert: [%s], err = %v\n", i+1, len(usedCerts), cert.Name, err2)
			failures = append(failures, fmt.Sprintf("[CAS] failed to upload cert [%s]: %v", cert.Name, err2))
			continue
		}

		casNameMap[cert.GetId()] = casName
	}

	for i, domain := range targets {
		casName, ok := casNameMap[domain.cert.GetId()]
		if !ok {
			fmt.Printf("  [%d/%d] [%s] Skipped domain: [%s], its cert: [%s] was not uploaded\n", i+1, len(targets), domain.product, domain.name, domain.cert.Name)
			failures = append(failures, fmt.Sprintf("[%s] skipped domain [%s], its cert [%s] was not uploaded", domain.product, domain.name, domain.cert.Name))
			continue
		}

		err = account.setDomainCert(domain, casName)
		if err != nil {
			fmt.Printf("  [%d/%d] [%s] Failed to set cert: [%s] for domain: [%s], err = %v\n", i+1, len(targets), domain.product, casName, domain.name, err)
			failures = append(failures, fmt.Sprintf("[%s] failed to set cert [%s] for domain [%s]: %v", domain.product, casName, domain.name, err))
			continue
		}

		fmt.Printf("  [%d/%d] [%s] Set cert: [%s] for domain: [%s]\n", i+1, len(targets), domain.product, casName, domain.name)
	}

	return failures
}

// isServiceNotOpenError tells whether the error only means that the account has not activated the
// product, like "CdnServiceNotFound". That account simply has no domain in that product, so it is
// not a failure.
func isServiceNotOpenError(err error) bool {
	if err == nil {
		return false
	}

	message := err.Error()
	keywords := []string{
		"ServiceNotFound",
		"is not activated",
		"does not open",
	}
	for _, keyword := range keywords {
		if strings.Contains(message, keyword) {
			return true
		}
	}

	return false
}

// matchCert returns the cert that covers the domain. Our certs are wildcard ones issued for
// "example.com" and "*.example.com", so a domain is covered when it is the cert's own domain or
// one of its direct sub domains.
func matchCert(domain string, certs []*Cert) *Cert {
	for _, cert := range certs {
		if cert.Name == domain {
			return cert
		}
	}

	index := strings.Index(domain, ".")
	if index == -1 {
		return nil
	}

	parent := domain[index+1:]
	for _, cert := range certs {
		if cert.Name == parent {
			return cert
		}
	}

	return nil
}

func (account *aliyunAccount) getCasCerts() ([]cas.Certificate, error) {
	res := []cas.Certificate{}
	for currentPage := 1; ; currentPage++ {
		request := cas.CreateDescribeUserCertificateListRequest()
		request.CurrentPage = requests.NewInteger(currentPage)
		request.ShowSize = requests.NewInteger(aliyunPageSize)

		response, err := account.casClient.DescribeUserCertificateList(request)
		if err != nil {
			return nil, err
		}

		res = append(res, response.CertificateList...)
		if len(response.CertificateList) == 0 || len(res) >= response.TotalCount {
			break
		}
	}

	return res, nil
}

// uploadCert uploads the cert to the SSL certificate service under an incremental name like
// "casnode.com-17". When the last uploaded cert already expires at the same date, it is the same
// cert, so it is reused instead of being uploaded again.
func (account *aliyunAccount) uploadCert(casCerts []cas.Certificate, cert *Cert) (string, error) {
	expireTime, err := getCertExpireTime(cert.Certificate)
	if err != nil {
		return "", err
	}

	latestCasCert, maxIndex := getLatestCasCert(casCerts, cert.Name)
	if latestCasCert != nil && isSameCertExpireTime(latestCasCert.EndDate, expireTime) {
		fmt.Printf("  [CAS] Cert: [%s] is already uploaded as [%s], reused it\n", cert.Name, latestCasCert.Name)
		return latestCasCert.Name, nil
	}

	name := fmt.Sprintf("%s-%d", cert.Name, maxIndex+1)

	request := cas.CreateCreateUserCertificateRequest()
	request.Name = name
	request.Cert = cert.Certificate
	request.Key = cert.PrivateKey

	response, err := account.casClient.CreateUserCertificate(request)
	if err != nil {
		return "", err
	}

	fmt.Printf("  [CAS] Uploaded cert: [%s] as [%s], certId = %d\n", cert.Name, name, response.CertId)
	return name, nil
}

// getLatestCasCert returns the uploaded cert with the largest index, e.g. "casnode.com-16" among
// "casnode.com-1" to "casnode.com-16", together with that index.
func getLatestCasCert(casCerts []cas.Certificate, certName string) (*cas.Certificate, int) {
	re := regexp.MustCompile(fmt.Sprintf(`^%s-(\d+)$`, regexp.QuoteMeta(certName)))

	var res *cas.Certificate
	maxIndex := 0
	for i := range casCerts {
		matches := re.FindStringSubmatch(casCerts[i].Name)
		if matches == nil {
			continue
		}

		index, err := strconv.Atoi(matches[1])
		if err != nil || (res != nil && index < maxIndex) {
			continue
		}

		maxIndex = index
		res = &casCerts[i]
	}

	return res, maxIndex
}

// isSameCertExpireTime compares the expire date of an uploaded Aliyun cert with the local one.
// Aliyun returns the date in several formats, so only the date part is compared.
func isSameCertExpireTime(endDate string, expireTime string) bool {
	if endDate == "" || expireTime == "" {
		return false
	}

	t, err := time.Parse(time.RFC3339, expireTime)
	if err != nil {
		return false
	}

	for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05", time.RFC3339} {
		endTime, err2 := time.Parse(layout, endDate)
		if err2 != nil {
			continue
		}

		return endTime.Format("2006-01-02") == t.Format("2006-01-02")
	}

	return false
}

func (account *aliyunAccount) setDomainCert(domain *aliyunDomain, casName string) error {
	switch domain.product {
	case "VOD":
		return account.setVodDomainCert(domain, casName)
	case "CDN":
		return account.setCdnDomainCert(domain, casName)
	case "DCDN":
		return account.setDcdnDomainCert(domain, casName)
	default:
		return fmt.Errorf("setDomainCert() error: unknown product: %s", domain.product)
	}
}

func (account *aliyunAccount) getVodDomains() ([]*aliyunDomain, error) {
	res := []*aliyunDomain{}
	for pageNumber := 1; ; pageNumber++ {
		request := vod.CreateDescribeVodUserDomainsRequest()
		request.PageNumber = requests.NewInteger(pageNumber)
		request.PageSize = requests.NewInteger(aliyunPageSize)

		response, err := account.vodClient.DescribeVodUserDomains(request)
		if err != nil {
			return nil, err
		}

		for _, pageData := range response.Domains.PageData {
			res = append(res, &aliyunDomain{
				product:     "VOD",
				name:        pageData.DomainName,
				sslProtocol: pageData.SslProtocol,
			})
		}

		if len(response.Domains.PageData) == 0 || int64(len(res)) >= response.TotalCount {
			break
		}
	}

	return res, nil
}

// setVodDomainCert updates the HTTPS config of a VOD domain. The VOD API has no "cas" cert type,
// so the cert content is sent along with the name that it was uploaded under.
func (account *aliyunAccount) setVodDomainCert(domain *aliyunDomain, casName string) error {
	request := vod.CreateSetVodDomainCertificateRequest()
	request.DomainName = domain.name
	request.CertName = casName
	request.SSLProtocol = "on"
	request.SSLPub = domain.cert.Certificate
	request.SSLPri = domain.cert.PrivateKey

	_, err := account.vodClient.SetVodDomainCertificate(request)
	return err
}

func (account *aliyunAccount) getCdnDomains() ([]*aliyunDomain, error) {
	res := []*aliyunDomain{}
	for pageNumber := 1; ; pageNumber++ {
		request := cdn.CreateDescribeUserDomainsRequest()
		request.PageNumber = requests.NewInteger(pageNumber)
		request.PageSize = requests.NewInteger(aliyunPageSize)

		response, err := account.cdnClient.DescribeUserDomains(request)
		if err != nil {
			return nil, err
		}

		for _, pageData := range response.Domains.PageData {
			res = append(res, &aliyunDomain{
				product:     "CDN",
				name:        pageData.DomainName,
				sslProtocol: pageData.SslProtocol,
			})
		}

		if len(response.Domains.PageData) == 0 || int64(len(res)) >= response.TotalCount {
			break
		}
	}

	return res, nil
}

// setCdnDomainCert updates the HTTPS config of a CDN domain. It first references the cert by the
// name that it was uploaded under, and falls back to sending the cert content when the CDN service
// cannot read it from the SSL certificate service.
func (account *aliyunAccount) setCdnDomainCert(domain *aliyunDomain, casName string) error {
	request := cdn.CreateSetDomainServerCertificateRequest()
	request.DomainName = domain.name
	request.CertName = casName
	request.CertType = "cas"
	request.ServerCertificateStatus = "on"
	request.ForceSet = "1"

	_, err := account.cdnClient.SetDomainServerCertificate(request)
	if err == nil {
		return nil
	}

	fmt.Printf("  [CDN] Failed to set cert: [%s] for domain: [%s] by name, retrying by uploading its content, err = %v\n", casName, domain.name, err)

	request = cdn.CreateSetDomainServerCertificateRequest()
	request.DomainName = domain.name
	request.CertName = casName
	request.CertType = "upload"
	request.ServerCertificateStatus = "on"
	request.ForceSet = "1"
	request.ServerCertificate = domain.cert.Certificate
	request.PrivateKey = domain.cert.PrivateKey

	_, err = account.cdnClient.SetDomainServerCertificate(request)
	return err
}

func (account *aliyunAccount) getDcdnDomains() ([]*aliyunDomain, error) {
	res := []*aliyunDomain{}
	for pageNumber := 1; ; pageNumber++ {
		request := dcdn.CreateDescribeDcdnUserDomainsRequest()
		request.PageNumber = requests.NewInteger(pageNumber)
		request.PageSize = requests.NewInteger(aliyunPageSize)

		response, err := account.dcdnClient.DescribeDcdnUserDomains(request)
		if err != nil {
			return nil, err
		}

		for _, pageData := range response.Domains.PageData {
			// DCDN returns the HTTPS switch under either of the two casings
			sslProtocol := pageData.SslProtocol
			if sslProtocol == "" {
				sslProtocol = pageData.SSLProtocol
			}

			res = append(res, &aliyunDomain{
				product:     "DCDN",
				name:        pageData.DomainName,
				sslProtocol: sslProtocol,
			})
		}

		if len(response.Domains.PageData) == 0 || int64(len(res)) >= response.TotalCount {
			break
		}
	}

	return res, nil
}

// setDcdnDomainCert updates the HTTPS config of a DCDN domain, it falls back the same way as
// setCdnDomainCert() does.
func (account *aliyunAccount) setDcdnDomainCert(domain *aliyunDomain, casName string) error {
	request := dcdn.CreateSetDcdnDomainCertificateRequest()
	request.DomainName = domain.name
	request.CertName = casName
	request.CertType = "cas"
	request.SSLProtocol = "on"
	request.ForceSet = "1"

	_, err := account.dcdnClient.SetDcdnDomainCertificate(request)
	if err == nil {
		return nil
	}

	fmt.Printf("  [DCDN] Failed to set cert: [%s] for domain: [%s] by name, retrying by uploading its content, err = %v\n", casName, domain.name, err)

	request = dcdn.CreateSetDcdnDomainCertificateRequest()
	request.DomainName = domain.name
	request.CertName = casName
	request.CertType = "upload"
	request.SSLProtocol = "on"
	request.ForceSet = "1"
	request.SSLPub = domain.cert.Certificate
	request.SSLPri = domain.cert.PrivateKey

	_, err = account.dcdnClient.SetDcdnDomainCertificate(request)
	return err
}
