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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/casbin/caswaf/conf"
	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/google/uuid"
)

type verifyResponse struct {
	Status string      `json:"status"`
	Msg    string      `json:"msg"`
	Sub    string      `json:"sub"`
	Name   string      `json:"name"`
	Data   bool        `json:"data"`
	Data2  interface{} `json:"data2"`
}

const captchaSessionTimeout = 30 * time.Minute

var (
	verifiedSession      = make(map[string]time.Time)
	verifiedSessionMutex sync.Mutex
)

func redirectToCaptcha(w http.ResponseWriter, r *http.Request) {
	scheme := getScheme(r)
	callbackUrl := fmt.Sprintf("%s://%s/caswaf-captcha-verify", scheme, r.Host)
	captchaUri := fmt.Sprintf(
		"%s/captcha?client_id=%s&redirect_uri=%s&state=%s",
		getCasdoorEndpoint(),
		conf.GetConfigString("clientId"),
		callbackUrl,
		conf.GetConfigString("casdoorApplication"),
	)
	http.Redirect(w, r, captchaUri, http.StatusFound)
}

func handleCaptchaCallback(w http.ResponseWriter, r *http.Request) {
	host := r.Host

	code := r.URL.Query().Get("code")
	typeStr := r.URL.Query().Get("type")
	secret := r.URL.Query().Get("secret")
	applicationId := r.URL.Query().Get("applicationId")
	if code == "" || typeStr == "" || secret == "" || applicationId == "" {
		redirectToCaptcha(w, r)
		return
	}

	var b bytes.Buffer
	writer := multipart.NewWriter(&b)
	_ = writer.WriteField("captchaToken", code)
	_ = writer.WriteField("clientSecret", secret)
	_ = writer.WriteField("applicationId", applicationId)
	_ = writer.WriteField("captchaType", typeStr)
	_ = writer.Close()
	verifyURL := casdoorsdk.GetUrl("verify-captcha", nil)
	req, err := http.NewRequest("POST", verifyURL, &b)
	if err != nil {
		redirectToCaptcha(w, r)
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		redirectToCaptcha(w, r)
		return
	}
	defer resp.Body.Close()
	// read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		redirectToCaptcha(w, r)
		return
	}
	// parse response
	var vr verifyResponse
	err = json.Unmarshal(body, &vr)
	if err != nil {
		redirectToCaptcha(w, r)
		return
	}
	if vr.Status != "ok" || !vr.Data {
		redirectToCaptcha(w, r)
		return
	}
	// set verified session
	uuidStr := uuid.NewString()
	addVerifiedSession(uuidStr)
	cookie := &http.Cookie{
		Name:       "casdoor_captcha_token",
		Value:      uuidStr,
		Path:       "/",
		Domain:     host,
		Expires:    time.Now().Add(captchaSessionTimeout),
		RawExpires: "",
		MaxAge:     0,
		Secure:     r.TLS != nil,
		HttpOnly:   true,
		SameSite:   0,
		Raw:        "",
		Unparsed:   nil,
	}

	originalPath := "/"
	if pathCookie, err := r.Cookie("casdoor_captcha_path"); err == nil && isLocalPath(pathCookie.Value) {
		originalPath = pathCookie.Value
	}

	http.SetCookie(w, cookie)
	http.SetCookie(w, &http.Cookie{Name: "casdoor_captcha_path", Path: "/", MaxAge: -1})
	http.Redirect(w, r, originalPath, http.StatusFound)
}

func addVerifiedSession(token string) {
	verifiedSessionMutex.Lock()
	defer verifiedSessionMutex.Unlock()

	now := time.Now()
	for t, createdTime := range verifiedSession {
		if now.Sub(createdTime) >= captchaSessionTimeout {
			delete(verifiedSession, t)
		}
	}
	verifiedSession[token] = now
}

// isLocalPath only lets the visitor be sent to a path of the same site, never to another host
func isLocalPath(path string) bool {
	return strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") && !strings.HasPrefix(path, "/\\")
}

func isVerifiedSession(r *http.Request) bool {
	cookie, err := r.Cookie("casdoor_captcha_token")
	if err != nil {
		return false
	}
	token := cookie.Value
	if token == "" {
		return false
	}

	verifiedSessionMutex.Lock()
	defer verifiedSessionMutex.Unlock()

	t, ok := verifiedSession[token]
	if ok {
		if time.Now().Sub(t) < captchaSessionTimeout {
			return true
		}
		delete(verifiedSession, token)
	}
	return false
}
