package discovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	aliyuncdn "github.com/aliyun/alibaba-cloud-sdk-go/services/cdn"
	"github.com/nonozone/TLSFerry/internal/credential"
	tencentcdn "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cdn/v20180606"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
)

func TestTencentScannerMapsDomainAndHTTPSState(t *testing.T) {
	client := &fakeTencentClient{response: &tencentcdn.DescribeDomainsConfigResponse{
		Response: &tencentcdn.DescribeDomainsConfigResponseParams{
			TotalNumber: common.Int64Ptr(1),
			Domains: []*tencentcdn.DetailDomain{{
				Domain: common.StringPtr("static.example.com"),
				Status: common.StringPtr("online"),
				Cname:  common.StringPtr("static.example.com.cdn.dnsv1.com"),
				Https:  &tencentcdn.Https{Switch: common.StringPtr("on")},
			}},
		},
	}}
	scanner := &tencentScanner{
		credentials: testResolver(map[string]string{"TEST_SECRET_ID": "id", "TEST_SECRET_KEY": "key"}),
		reference:   "env:TEST",
		newClient:   func(string, string) (tencentCDNClient, error) { return client, nil },
	}

	domains, err := scanner.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(domains) != 1 || domains[0].Name != "static.example.com" || !domains[0].HTTPS || domains[0].Status != "online" {
		t.Fatalf("Scan() = %#v", domains)
	}
}

func TestAliyunScannerMapsDomainAndHTTPSState(t *testing.T) {
	response := aliyuncdn.CreateDescribeUserDomainsResponse()
	response.TotalCount = 1
	response.Domains.PageData = []aliyuncdn.PageData{{
		DomainName:   "assets.example.com",
		DomainStatus: "online",
		SslProtocol:  "on",
		Cname:        "assets.example.com.w.kunlunca.com",
	}}
	scanner := &aliyunScanner{
		credentials: testResolver(map[string]string{"TEST_ACCESS_KEY_ID": "id", "TEST_ACCESS_KEY_SECRET": "key"}),
		reference:   "env:TEST",
		newClient: func(string, string) (aliyunCDNClient, error) {
			return &fakeAliyunClient{response: response}, nil
		},
	}

	domains, err := scanner.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(domains) != 1 || domains[0].Name != "assets.example.com" || !domains[0].HTTPS {
		t.Fatalf("Scan() = %#v", domains)
	}
}

func TestQiniuScannerUsesSignedReadOnlyDomainRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		requests++
		if request.Method != http.MethodGet || request.URL.Path != "/domain" {
			t.Errorf("request = %s %s", request.Method, request.URL.Path)
		}
		if request.URL.Query().Get("limit") != "1000" {
			t.Errorf("limit = %q", request.URL.Query().Get("limit"))
		}
		if !strings.HasPrefix(request.Header.Get("Authorization"), "QBox ") {
			t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
		}
		response.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			_, _ = response.Write([]byte(`{"marker":"next-page","domains":[{"name":"cdn.example.com","operatingState":"online","cname":"cdn.example.com.qiniudns.com","https":{"certId":"cert-1"}}]}`))
			return
		}
		if request.URL.Query().Get("marker") != "next-page" {
			t.Errorf("marker = %q", request.URL.Query().Get("marker"))
		}
		_, _ = response.Write([]byte(`{"domains":[{"name":"img.example.com","operatingState":"offlined"}]}`))
	}))
	defer server.Close()
	scanner := &qiniuScanner{
		credentials: testResolver(map[string]string{"TEST_ACCESS_KEY": "id", "TEST_SECRET_KEY": "key"}),
		reference:   "env:TEST",
		baseURL:     server.URL,
		client:      server.Client(),
	}

	domains, err := scanner.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(domains) != 2 || domains[0].Name != "cdn.example.com" || !domains[0].HTTPS || domains[0].Status != "online" || domains[1].Name != "img.example.com" {
		t.Fatalf("Scan() = %#v", domains)
	}
}

type fakeTencentClient struct {
	response *tencentcdn.DescribeDomainsConfigResponse
}

func (c *fakeTencentClient) DescribeDomainsConfigWithContext(context.Context, *tencentcdn.DescribeDomainsConfigRequest) (*tencentcdn.DescribeDomainsConfigResponse, error) {
	return c.response, nil
}

type fakeAliyunClient struct {
	response *aliyuncdn.DescribeUserDomainsResponse
}

func (c *fakeAliyunClient) DescribeUserDomains(*aliyuncdn.DescribeUserDomainsRequest) (*aliyuncdn.DescribeUserDomainsResponse, error) {
	return c.response, nil
}

func testResolver(values map[string]string) credential.Resolver {
	return credential.Resolver{Lookup: func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}}
}
