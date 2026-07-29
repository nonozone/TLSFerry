package deployment

import (
	"context"
	"testing"

	"github.com/nonozone/TLSFerry/internal/certstore"
	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/credential"
	ssl "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/ssl/v20191205"
)

type fakeTencentClient struct {
	upload *ssl.UploadCertificateRequest
	deploy *ssl.DeployCertificateInstanceRequest
}

func (f *fakeTencentClient) UploadCertificateWithContext(_ context.Context, request *ssl.UploadCertificateRequest) (*ssl.UploadCertificateResponse, error) {
	f.upload = request
	return &ssl.UploadCertificateResponse{Response: &ssl.UploadCertificateResponseParams{RepeatCertId: stringPointer("cert-1")}}, nil
}

func (f *fakeTencentClient) DeployCertificateInstanceWithContext(_ context.Context, request *ssl.DeployCertificateInstanceRequest) (*ssl.DeployCertificateInstanceResponse, error) {
	f.deploy = request
	return &ssl.DeployCertificateInstanceResponse{Response: &ssl.DeployCertificateInstanceResponseParams{DeployRecordId: uint64Pointer(42)}}, nil
}

func TestTencentDeploysToCDN(t *testing.T) {
	fake := &fakeTencentClient{}
	provider := tencentProvider{
		credentials: credential.EnvResolver{Lookup: credentialLookup(map[string]string{
			"TENCENTCLOUD_SECRET_ID": "id", "TENCENTCLOUD_SECRET_KEY": "key",
		})},
		newClient: func(string, string, string) (tencentSSLClient, error) { return fake, nil },
	}
	result, err := provider.Deploy(context.Background(), "assets", config.Deployment{
		Provider: "tencent-cdn", Target: "assets.example.com", Credential: "env:TENCENTCLOUD",
	}, certstore.Bundle{Certificate: []byte("cert"), PrivateKey: []byte("key")})
	if err != nil {
		t.Fatalf("Deploy() returned an unexpected error: %v", err)
	}
	if result.Reference != "42" || result.Status != "submitted" {
		t.Fatalf("Deploy() result = %#v", result)
	}
	if fake.deploy.ResourceType == nil || *fake.deploy.ResourceType != "cdn" || len(fake.deploy.InstanceIdList) != 1 || *fake.deploy.InstanceIdList[0] != "assets.example.com|on" {
		t.Fatalf("deploy request = %#v", fake.deploy)
	}
}

func stringPointer(value string) *string { return &value }
func uint64Pointer(value uint64) *uint64 { return &value }

func credentialLookup(values map[string]string) credential.LookupEnv {
	return func(name string) (string, bool) { value, ok := values[name]; return value, ok }
}
