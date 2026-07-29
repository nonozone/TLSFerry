package deployment

import (
	"context"
	"fmt"

	"github.com/nonozone/TLSFerry/internal/certstore"
	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/credential"
)

type Result struct {
	Provider  string
	Target    string
	Reference string
	Status    string
}

func NewManager(credentials credential.EnvResolver) Manager {
	return Manager{providers: map[string]Provider{
		"tencent-cdn": tencentProvider{credentials: credentials},
		"tencent-cos": tencentProvider{credentials: credentials},
		"aliyun-cdn":  aliyunProvider{credentials: credentials},
		"qiniu-cdn":   qiniuProvider{credentials: credentials},
	}}
}

type Provider interface {
	Deploy(context.Context, string, config.Deployment, certstore.Bundle) (Result, error)
}

type Manager struct {
	providers map[string]Provider
}

func (m Manager) Deploy(ctx context.Context, certificateName string, deployment config.Deployment, bundle certstore.Bundle) (Result, error) {
	provider, ok := m.providers[deployment.Provider]
	if !ok {
		return Result{}, fmt.Errorf("unsupported deployment provider %q", deployment.Provider)
	}
	result, err := provider.Deploy(ctx, certificateName, deployment, bundle)
	if err != nil {
		return Result{}, fmt.Errorf("deploy %q to %s: %w", certificateName, deployment.Provider, err)
	}
	return result, nil
}
