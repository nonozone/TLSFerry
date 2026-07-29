package deployment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nonozone/TLSFerry/internal/certstore"
	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/credential"
	qiniuauth "github.com/qiniu/go-sdk/v7/auth"
)

type qiniuProvider struct {
	credentials credential.EnvResolver
	baseURL     string
	client      *http.Client
}

func (p qiniuProvider) Deploy(ctx context.Context, certificateName string, deployment config.Deployment, bundle certstore.Bundle) (Result, error) {
	if err := safeTarget(deployment.Target); err != nil {
		return Result{}, err
	}
	values, err := p.credentials.Values(deployment.Credential, "ACCESS_KEY", "SECRET_KEY")
	if err != nil {
		return Result{}, err
	}
	credentials := qiniuauth.New(values["ACCESS_KEY"], values["SECRET_KEY"])
	upload := struct {
		Name       string `json:"name"`
		CommonName string `json:"common_name"`
		CA         string `json:"ca"`
		PrivateKey string `json:"pri"`
	}{"TLSFerry-" + certificateName, deployment.Target, string(bundle.FullChain()), string(bundle.PrivateKey)}
	var uploadResponse map[string]any
	if err := p.request(ctx, credentials, http.MethodPost, "/sslcert", upload, &uploadResponse); err != nil {
		return Result{}, fmt.Errorf("upload certificate to Qiniu: %w", err)
	}
	certificateID, _ := uploadResponse["certID"].(string)
	if certificateID == "" {
		certificateID, _ = uploadResponse["certid"].(string)
	}
	if certificateID == "" {
		return Result{}, errors.New("Qiniu certificate upload returned no certificate ID")
	}

	var domain struct {
		HTTPS struct {
			CertificateID string `json:"certId"`
			ForceHTTPS    bool   `json:"forceHttps"`
			HTTP2Enable   bool   `json:"http2Enable"`
		} `json:"https"`
	}
	domainPath := "/domain/" + url.PathEscape(deployment.Target)
	if err := p.request(ctx, credentials, http.MethodGet, domainPath, nil, &domain); err != nil {
		return Result{}, fmt.Errorf("read Qiniu CDN domain: %w", err)
	}
	forceHTTPS := optionBool(deployment.Options, "force_https", domain.HTTPS.ForceHTTPS)
	http2 := optionBool(deployment.Options, "http2", true)
	if domain.HTTPS.CertificateID != "" && deployment.Options["http2"] == "" {
		http2 = domain.HTTPS.HTTP2Enable
	}
	body := map[string]any{"certid": certificateID, "forceHttps": forceHTTPS, "http2Enable": http2}
	endpoint := domainPath + "/httpsconf"
	if domain.HTTPS.CertificateID == "" {
		endpoint = domainPath + "/sslize"
	}
	if err := p.request(ctx, credentials, http.MethodPut, endpoint, body, nil); err != nil {
		return Result{}, fmt.Errorf("update Qiniu CDN HTTPS configuration: %w", err)
	}
	return Result{Provider: deployment.Provider, Target: deployment.Target, Reference: certificateID, Status: "applied"}, nil
}

func (p qiniuProvider) request(ctx context.Context, credentials *qiniuauth.Credentials, method, path string, body, output any) error {
	baseURL := p.baseURL
	if baseURL == "" {
		baseURL = "https://api.qiniu.com"
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return err
	}
	if body == nil {
		bodyBytes = nil
	}
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(baseURL, "/")+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	token, err := credentials.SignRequest(request)
	if err != nil {
		return fmt.Errorf("sign Qiniu request: %w", err)
	}
	request.Header.Set("Authorization", "QBox "+token)
	client := p.client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Qiniu API returned %s", response.Status)
	}
	if output != nil && len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, output); err != nil {
			return fmt.Errorf("decode Qiniu response: %w", err)
		}
	}
	return nil
}

func optionBool(options map[string]string, name string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(options[name])) {
	case "true", "on", "1", "yes":
		return true
	case "false", "off", "0", "no":
		return false
	default:
		return fallback
	}
}
