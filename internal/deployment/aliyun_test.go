package deployment

import (
	"context"
	"testing"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/cdn"
	"github.com/nonozone/TLSFerry/internal/certstore"
	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/credential"
)

type fakeAliyunClient struct {
	request *cdn.SetCdnDomainSSLCertificateRequest
}

func (f *fakeAliyunClient) SetCdnDomainSSLCertificate(request *cdn.SetCdnDomainSSLCertificateRequest) (*cdn.SetCdnDomainSSLCertificateResponse, error) {
	f.request = request
	return &cdn.SetCdnDomainSSLCertificateResponse{RequestId: "request-1"}, nil
}

func TestAliyunDeploysToCDN(t *testing.T) {
	fake := &fakeAliyunClient{}
	provider := aliyunProvider{
		credentials: credential.EnvResolver{Lookup: credentialLookup(map[string]string{
			"ALIYUN_ACCESS_KEY_ID": "id", "ALIYUN_ACCESS_KEY_SECRET": "key",
		})},
		newClient: func(string, string, string) (aliyunCDNClient, error) { return fake, nil },
	}
	result, err := provider.Deploy(context.Background(), "assets", config.Deployment{
		Provider: "aliyun-cdn", Target: "assets.example.com", Credential: "env:ALIYUN",
	}, certstore.Bundle{Certificate: []byte("cert"), PrivateKey: []byte("key")})
	if err != nil {
		t.Fatalf("Deploy() returned an unexpected error: %v", err)
	}
	if result.Reference != "request-1" || fake.request.DomainName != "assets.example.com" || fake.request.CertType != "upload" || fake.request.SSLProtocol != "on" {
		t.Fatalf("result = %#v, request = %#v", result, fake.request)
	}
}
