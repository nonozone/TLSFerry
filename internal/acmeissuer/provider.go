package acmeissuer

import (
	"fmt"

	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/providers/dns/alidns"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/providers/dns/tencentcloud"
	"github.com/nonozone/TLSFerry/internal/credential"
)

type providerFactory struct {
	credentials credential.EnvResolver
}

func (f providerFactory) new(name, reference string) (challenge.Provider, error) {
	switch name {
	case "cloudflare":
		values, err := f.credentials.Values(reference, "API_TOKEN")
		if err != nil {
			return nil, fmt.Errorf("cloudflare DNS credentials: %w", err)
		}
		providerConfig := cloudflare.NewDefaultConfig()
		providerConfig.AuthToken = values["API_TOKEN"]
		providerConfig.ZoneToken = values["API_TOKEN"]
		provider, err := cloudflare.NewDNSProviderConfig(providerConfig)
		if err != nil {
			return nil, fmt.Errorf("create cloudflare provider: %w", err)
		}
		return provider, nil

	case "dnspod":
		values, err := f.credentials.Values(reference, "SECRET_ID", "SECRET_KEY")
		if err != nil {
			return nil, fmt.Errorf("dnspod credentials: %w", err)
		}
		providerConfig := tencentcloud.NewDefaultConfig()
		providerConfig.SecretID = values["SECRET_ID"]
		providerConfig.SecretKey = values["SECRET_KEY"]
		provider, err := tencentcloud.NewDNSProviderConfig(providerConfig)
		if err != nil {
			return nil, fmt.Errorf("create dnspod provider: %w", err)
		}
		return provider, nil

	case "aliyun":
		values, err := f.credentials.Values(reference, "ACCESS_KEY_ID", "ACCESS_KEY_SECRET")
		if err != nil {
			return nil, fmt.Errorf("aliyun DNS credentials: %w", err)
		}
		providerConfig := alidns.NewDefaultConfig()
		providerConfig.APIKey = values["ACCESS_KEY_ID"]
		providerConfig.SecretKey = values["ACCESS_KEY_SECRET"]
		provider, err := alidns.NewDNSProviderConfig(providerConfig)
		if err != nil {
			return nil, fmt.Errorf("create aliyun DNS provider: %w", err)
		}
		return provider, nil

	default:
		return nil, fmt.Errorf("unsupported DNS provider %q", name)
	}
}
