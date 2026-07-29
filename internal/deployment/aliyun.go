package deployment

import (
	"context"
	"fmt"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/cdn"
	"github.com/nonozone/TLSFerry/internal/certstore"
	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/credential"
)

type aliyunCDNClient interface {
	SetCdnDomainSSLCertificate(*cdn.SetCdnDomainSSLCertificateRequest) (*cdn.SetCdnDomainSSLCertificateResponse, error)
}

type aliyunProvider struct {
	credentials credential.EnvResolver
	newClient   func(string, string, string) (aliyunCDNClient, error)
}

func (p aliyunProvider) Deploy(ctx context.Context, certificateName string, deployment config.Deployment, bundle certstore.Bundle) (Result, error) {
	if err := safeTarget(deployment.Target); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	values, err := p.credentials.Values(deployment.Credential, "ACCESS_KEY_ID", "ACCESS_KEY_SECRET")
	if err != nil {
		return Result{}, err
	}
	region := deployment.Options["region"]
	if region == "" {
		region = "cn-hangzhou"
	}
	newClient := p.newClient
	if newClient == nil {
		newClient = func(region, accessKeyID, accessKeySecret string) (aliyunCDNClient, error) {
			return cdn.NewClientWithAccessKey(region, accessKeyID, accessKeySecret)
		}
	}
	client, err := newClient(region, values["ACCESS_KEY_ID"], values["ACCESS_KEY_SECRET"])
	if err != nil {
		return Result{}, fmt.Errorf("create Alibaba Cloud CDN client: %w", err)
	}
	request := cdn.CreateSetCdnDomainSSLCertificateRequest()
	request.DomainName = deployment.Target
	request.SSLProtocol = "on"
	request.CertType = "upload"
	request.CertName = "TLSFerry-" + certificateName
	request.SSLPub = string(bundle.FullChain())
	request.SSLPri = string(bundle.PrivateKey)
	response, err := client.SetCdnDomainSSLCertificate(request)
	if err != nil {
		return Result{}, fmt.Errorf("update Alibaba Cloud CDN certificate: %w", err)
	}
	if response == nil || response.RequestId == "" {
		return Result{}, fmt.Errorf("Alibaba Cloud CDN returned no request ID")
	}
	return Result{Provider: deployment.Provider, Target: deployment.Target, Reference: response.RequestId, Status: "applied"}, nil
}
