package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nonozone/TLSFerry/internal/credential"
	qiniuauth "github.com/qiniu/go-sdk/v7/auth"
)

type qiniuScanner struct {
	credentials credential.Resolver
	reference   string
	baseURL     string
	client      *http.Client
}

type qiniuDomain struct {
	Name           string `json:"name"`
	Domain         string `json:"domain"`
	CNAME          string `json:"cname"`
	OperatingState string `json:"operatingState"`
	State          string `json:"state"`
	HTTPS          *struct {
		CertificateID string `json:"certId"`
	} `json:"https"`
}

func (s *qiniuScanner) Scan(ctx context.Context) ([]Domain, error) {
	values, err := s.credentials.Values(s.reference, "ACCESS_KEY", "SECRET_KEY")
	if err != nil {
		return nil, err
	}
	baseURL := s.baseURL
	if baseURL == "" {
		baseURL = "https://api.qiniu.com"
	}
	client := s.client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	credentials := qiniuauth.New(values["ACCESS_KEY"], values["SECRET_KEY"])
	var domains []Domain
	marker := ""
	for {
		query := url.Values{"limit": []string{"1000"}}
		if marker != "" {
			query.Set("marker", marker)
		}
		endpoint := strings.TrimRight(baseURL, "/") + "/domain?" + query.Encode()
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		token, err := credentials.SignRequest(request)
		if err != nil {
			return nil, fmt.Errorf("sign Qiniu request: %w", err)
		}
		request.Header.Set("Authorization", "QBox "+token)
		response, err := client.Do(request)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 8<<20))
		response.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, fmt.Errorf("Qiniu API returned %s: %s", response.Status, strings.TrimSpace(string(body)))
		}
		items, nextMarker, err := decodeQiniuDomains(body)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			name := item.Name
			if name == "" {
				name = item.Domain
			}
			if name == "" {
				continue
			}
			status := item.OperatingState
			if status == "" {
				status = item.State
			}
			domains = append(domains, Domain{Provider: "qiniu", Name: name, Status: status, HTTPS: item.HTTPS != nil, CNAME: item.CNAME})
		}
		if nextMarker == "" || nextMarker == marker {
			break
		}
		marker = nextMarker
	}
	return domains, nil
}

func decodeQiniuDomains(body []byte) ([]qiniuDomain, string, error) {
	var objects []qiniuDomain
	if err := json.Unmarshal(body, &objects); err == nil {
		return objects, "", nil
	}
	var names []string
	if err := json.Unmarshal(body, &names); err == nil {
		objects = make([]qiniuDomain, 0, len(names))
		for _, name := range names {
			objects = append(objects, qiniuDomain{Name: name})
		}
		return objects, "", nil
	}
	var wrapper struct {
		Domains json.RawMessage `json:"domains"`
		Marker  string          `json:"marker"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil || len(wrapper.Domains) == 0 {
		return nil, "", fmt.Errorf("decode Qiniu domain list")
	}
	if err := json.Unmarshal(wrapper.Domains, &objects); err == nil {
		return objects, wrapper.Marker, nil
	}
	if err := json.Unmarshal(wrapper.Domains, &names); err == nil {
		objects = make([]qiniuDomain, 0, len(names))
		for _, name := range names {
			objects = append(objects, qiniuDomain{Name: name})
		}
		return objects, wrapper.Marker, nil
	}
	return nil, "", fmt.Errorf("decode Qiniu domain list")
}
