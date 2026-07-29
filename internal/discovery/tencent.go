package discovery

import (
	"context"
	"fmt"

	"github.com/nonozone/TLSFerry/internal/credential"
	cdn "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cdn/v20180606"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
)

type tencentCDNClient interface {
	DescribeDomainsConfigWithContext(context.Context, *cdn.DescribeDomainsConfigRequest) (*cdn.DescribeDomainsConfigResponse, error)
}

type tencentScanner struct {
	credentials credential.Resolver
	reference   string
	newClient   func(string, string) (tencentCDNClient, error)
}

func (s *tencentScanner) Scan(ctx context.Context) ([]Domain, error) {
	values, err := s.credentials.Values(s.reference, "SECRET_ID", "SECRET_KEY")
	if err != nil {
		return nil, err
	}
	newClient := s.newClient
	if newClient == nil {
		newClient = func(secretID, secretKey string) (tencentCDNClient, error) {
			return cdn.NewClient(common.NewCredential(secretID, secretKey), "", profile.NewClientProfile())
		}
	}
	client, err := newClient(values["SECRET_ID"], values["SECRET_KEY"])
	if err != nil {
		return nil, fmt.Errorf("create Tencent Cloud CDN client: %w", err)
	}
	const limit int64 = 100
	var domains []Domain
	for offset := int64(0); ; offset += limit {
		request := cdn.NewDescribeDomainsConfigRequest()
		request.Offset = common.Int64Ptr(offset)
		request.Limit = common.Int64Ptr(limit)
		response, err := client.DescribeDomainsConfigWithContext(ctx, request)
		if err != nil {
			return nil, err
		}
		if response == nil || response.Response == nil {
			return nil, fmt.Errorf("Tencent Cloud CDN returned an empty response")
		}
		for _, item := range response.Response.Domains {
			if item == nil || item.Domain == nil || *item.Domain == "" {
				continue
			}
			domain := Domain{Provider: "tencent", Name: *item.Domain}
			if item.Status != nil {
				domain.Status = *item.Status
			}
			if item.Cname != nil {
				domain.CNAME = *item.Cname
			}
			domain.HTTPS = item.Https != nil && item.Https.Switch != nil && *item.Https.Switch == "on"
			domains = append(domains, domain)
		}
		if int64(len(response.Response.Domains)) < limit || (response.Response.TotalNumber != nil && int64(len(domains)) >= *response.Response.TotalNumber) {
			break
		}
	}
	return domains, nil
}
