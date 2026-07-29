package acmeissuer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/challenge/dns01"
)

const remoteDNSProtocolVersion = "1"

var remoteErrorCodePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

type remoteChallengeRequest struct {
	Domain        string `json:"domain"`
	FQDN          string `json:"fqdn"`
	EffectiveFQDN string `json:"effective_fqdn"`
	Value         string `json:"value"`
}

type remoteDNSProvider struct {
	baseURL       string
	apiToken      string
	httpClient    *http.Client
	challengeInfo func(domain, keyAuth string) dns01.ChallengeInfo
}

func newRemoteDNSProvider(rawURL, apiToken string, client *http.Client) (*remoteDNSProvider, error) {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return nil, fmt.Errorf("remote DNS API URL is invalid: %q", rawURL)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("remote DNS API URL cannot contain user info, query parameters, or a fragment")
	}
	if parsed.Scheme != "https" && !isLoopbackHost(parsed.Hostname()) {
		return nil, errors.New("remote DNS API URL must use HTTPS")
	}
	apiToken = strings.TrimSpace(apiToken)
	if apiToken == "" {
		return nil, errors.New("remote DNS API token is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	return &remoteDNSProvider{
		baseURL:       strings.TrimRight(parsed.String(), "/"),
		apiToken:      apiToken,
		httpClient:    client,
		challengeInfo: dns01.GetChallengeInfo,
	}, nil
}

func (p *remoteDNSProvider) Present(domain, token, keyAuth string) error {
	info := p.challengeInfo(domain, keyAuth)
	payload := remoteChallengeRequest{
		Domain:        domain,
		FQDN:          info.FQDN,
		EffectiveFQDN: info.EffectiveFQDN,
		Value:         info.Value,
	}
	return p.send(http.MethodPut, challengeResourcePath(info.FQDN, token), payload)
}

func (p *remoteDNSProvider) CleanUp(domain, token, keyAuth string) error {
	info := p.challengeInfo(domain, keyAuth)
	err := p.send(http.MethodDelete, challengeResourcePath(info.FQDN, token), nil)
	var statusErr *remoteDNSStatusError
	if errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
		return nil
	}
	return err
}

func (p *remoteDNSProvider) Timeout() (time.Duration, time.Duration) {
	return 2 * time.Minute, 2 * time.Second
}

func (p *remoteDNSProvider) send(method, path string, payload any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode remote DNS challenge: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, p.baseURL+path, body)
	if err != nil {
		return fmt.Errorf("create remote DNS challenge request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+p.apiToken)
	request.Header.Set("X-TLSFerry-Protocol-Version", remoteDNSProtocolVersion)
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := p.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("send remote DNS challenge request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	var envelope struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(message, &envelope) != nil || !remoteErrorCodePattern.MatchString(envelope.Code) {
		envelope.Code = ""
	}
	return &remoteDNSStatusError{StatusCode: response.StatusCode, Code: envelope.Code}
}

type remoteDNSStatusError struct {
	StatusCode int
	Code       string
}

func (e *remoteDNSStatusError) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("remote DNS control plane returned HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("remote DNS control plane returned HTTP %d (%s)", e.StatusCode, e.Code)
}

func challengeResourcePath(fqdn, token string) string {
	digest := sha256.Sum256([]byte(fqdn + "\x00" + token))
	return "/v1/acme-challenges/" + hex.EncodeToString(digest[:])
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
