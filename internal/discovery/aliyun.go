package discovery

import (
	"context"
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/sdk/requests"
	aliyuncdn "github.com/aliyun/alibaba-cloud-sdk-go/services/cdn"
	"github.com/nonozone/TLSFerry/internal/credential"
)

type aliyunCDNClient interface {
	DescribeUserDomains(*aliyuncdn.DescribeUserDomainsRequest) (*aliyuncdn.DescribeUserDomainsResponse, error)
}

type aliyunScanner struct {
	credentials credential.Resolver
	reference   string
	newClient   func(string, string) (aliyunCDNClient, error)
}

func (s *aliyunScanner) Scan(ctx context.Context) ([]Domain, error) {
	values, err := s.credentials.Values(s.reference, "ACCESS_KEY_ID", "ACCESS_KEY_SECRET")
	if err != nil {
		return nil, err
	}
	newClient := s.newClient
	if newClient == nil {
		newClient = func(accessKeyID, accessKeySecret string) (aliyunCDNClient, error) {
			return aliyuncdn.NewClientWithAccessKey("cn-hangzhou", accessKeyID, accessKeySecret)
		}
	}
	client, err := newClient(values["ACCESS_KEY_ID"], values["ACCESS_KEY_SECRET"])
	if err != nil {
		return nil, fmt.Errorf("create Alibaba Cloud CDN client: %w", err)
	}
	const pageSize = 500
	var domains []Domain
	for page := 1; ; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		request := aliyuncdn.CreateDescribeUserDomainsRequest()
		request.PageNumber = requests.NewInteger(page)
		request.PageSize = requests.NewInteger(pageSize)
		response, err := client.DescribeUserDomains(request)
		if err != nil {
			return nil, err
		}
		if response == nil {
			return nil, fmt.Errorf("Alibaba Cloud CDN returned an empty response")
		}
		for _, item := range response.Domains.PageData {
			if item.DomainName == "" {
				continue
			}
			domains = append(domains, Domain{
				Provider: "aliyun",
				Name:     item.DomainName,
				Status:   item.DomainStatus,
				HTTPS:    item.SslProtocol == "on",
				CNAME:    item.Cname,
			})
		}
		if len(response.Domains.PageData) < pageSize || (response.TotalCount > 0 && int64(len(domains)) >= response.TotalCount) {
			break
		}
	}
	return domains, nil
}
