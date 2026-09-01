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
	"time"

	"github.com/casbin/caswaf/certificate"
	"github.com/casbin/caswaf/util"
	"github.com/xorm-io/core"
)

type Cert struct {
	Owner       string `xorm:"varchar(100) notnull pk" json:"owner"`
	Name        string `xorm:"varchar(100) notnull pk" json:"name"`
	CreatedTime string `xorm:"varchar(100)" json:"createdTime"`
	DisplayName string `xorm:"varchar(100)" json:"displayName"`

	Type             string `xorm:"varchar(100)" json:"type"`
	CryptoAlgorithm  string `xorm:"varchar(100)" json:"cryptoAlgorithm"`
	ExpireTime       string `xorm:"varchar(100)" json:"expireTime"`
	DomainExpireTime string `xorm:"varchar(100)" json:"domainExpireTime"`

	Provider     string `xorm:"varchar(100)" json:"provider"`
	Account      string `xorm:"varchar(100)" json:"account"`
	AccessKey    string `xorm:"varchar(100)" json:"accessKey"`
	AccessSecret string `xorm:"varchar(100)" json:"accessSecret"`

	Certificate string `xorm:"mediumtext" json:"certificate"`
	PrivateKey  string `xorm:"mediumtext" json:"privateKey"`
}

func GetGlobalCerts() ([]*Cert, error) {
	certs := []*Cert{}
	err := ormer.Engine.Asc("owner").Desc("created_time").Find(&certs)
	return certs, err
}

func GetCerts(owner string) ([]*Cert, error) {
	certs := []*Cert{}
	err := ormer.Engine.Desc("created_time").Find(&certs, &Cert{Owner: owner})
	if err != nil {
		return nil, err
	}

	for _, cert := range certs {
		if cert.Certificate != "" && cert.ExpireTime == "" {
			cert.ExpireTime, err = getCertExpireTime(cert.Certificate)
			if err != nil {
				return nil, err
			}

			_, err = UpdateCert(cert.GetId(), cert)
			if err != nil {
				return nil, err
			}
		}
	}

	return certs, nil
}

func getCert(owner string, name string) (*Cert, error) {
	cert := Cert{Owner: owner, Name: name}
	existed, err := ormer.Engine.Get(&cert)
	if err != nil {
		return nil, err
	}

	if existed {
		return &cert, nil
	} else {
		return nil, nil
	}
}

func GetCert(id string) (*Cert, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	return getCert(owner, name)
}

func GetMaskedCert(cert *Cert) *Cert {
	if cert == nil {
		return nil
	}

	if cert.AccessSecret != "" {
		cert.AccessSecret = "***"
	}

	return cert
}

func GetMaskedCerts(certs []*Cert) []*Cert {
	for _, cert := range certs {
		cert = GetMaskedCert(cert)
	}
	return certs
}

func UpdateCert(id string, cert *Cert) (bool, error) {
	owner, name := util.GetOwnerAndNameFromId(id)
	if c, err := getCert(owner, name); err != nil || c == nil {
		return false, err
	}

	if cert.Certificate != "" {
		expireTime, err := getCertExpireTime(cert.Certificate)
		if err != nil {
			return false, err
		}

		cert.ExpireTime = expireTime
	} else {
		cert.ExpireTime = ""
	}

	_, err := ormer.Engine.ID(core.PK{owner, name}).AllCols().Update(cert)
	if err != nil {
		return false, err
	}

	return true, nil
}

func AddCert(cert *Cert) (bool, error) {
	affected, err := ormer.Engine.Insert(cert)
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func DeleteCert(cert *Cert) (bool, error) {
	affected, err := ormer.Engine.ID(core.PK{cert.Owner, cert.Name}).Delete(&Cert{})
	if err != nil {
		return false, err
	}

	return affected != 0, nil
}

func (cert *Cert) GetId() string {
	return fmt.Sprintf("%s/%s", cert.Owner, cert.Name)
}

func RenewCert(cert *Cert) (bool, error) {
	useProxy := true
	if cert.Provider == "GoDaddy" {
		useProxy = true
	}

	client, err := GetAcmeClient(useProxy)
	if err != nil {
		return false, err
	}

	var certStr, privateKey string
	if cert.Provider == "Aliyun" {
		certStr, privateKey, err = certificate.ObtainCertificateAli(client, cert.Name, cert.AccessKey, cert.AccessSecret)
	} else if cert.Provider == "GoDaddy" {
		certStr, privateKey, err = certificate.ObtainCertificateGoDaddy(client, cert.Name, cert.AccessKey, cert.AccessSecret)
	} else {
		return false, fmt.Errorf("unknown provider: %s", cert.Provider)
	}
	if err != nil {
		return false, err
	}

	expireTime, err := getCertExpireTime(certStr)
	if err != nil {
		return false, err
	}

	cert.ExpireTime = expireTime
	cert.Certificate = certStr
	cert.PrivateKey = privateKey

	return UpdateCert(cert.GetId(), cert)
}

func (cert *Cert) isCertNearExpire() (bool, error) {
	if cert.ExpireTime == "" {
		return true, nil
	}

	expireTime, err := time.Parse(time.RFC3339, cert.ExpireTime)
	if err != nil {
		return false, err
	}

	now := time.Now()
	duration := expireTime.Sub(now)
	res := duration <= 7*24*time.Hour

	return res, nil
}

func GetCertCount(owner, field, value string) (int64, error) {
	session := GetSession(owner, -1, -1, field, value, "", "")
	return session.Count(&Cert{})
}

func GetPaginationCerts(owner string, offset, limit int, field, value, sortField, sortOrder string) ([]*Cert, error) {
	certs := []*Cert{}
	session := GetSession(owner, offset, limit, field, value, sortField, sortOrder)
	err := session.Where("owner = ? or owner = ?", "admin", owner).Find(&certs)
	if err != nil {
		return certs, err
	}

	return certs, nil
}
