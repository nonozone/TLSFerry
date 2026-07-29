package acmeissuer

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/providers/dns/alidns"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/providers/dns/tencentcloud"
	"github.com/nonozone/TLSFerry/internal/credential"
)

type providerFactory struct {
	credentials credential.EnvResolver
}

type DNSControlChecker struct {
	Credentials credential.Resolver
	Random      io.Reader
	newProvider func(name, reference string) (challenge.Provider, error)
}

func (c DNSControlChecker) Check(name, reference, domain string) error {
	fields, ok := directDNSProviderCredentialFields(name)
	if !ok {
		if name == "tlsferry-cloud" {
			return fmt.Errorf("DNS provider %q uses a job-scoped remote protocol and cannot be diagnosed independently", name)
		}
		return fmt.Errorf("unsupported DNS provider %q", name)
	}
	values, err := c.Credentials.Values(reference, fields...)
	if err != nil {
		return fmt.Errorf("%s DNS credentials: %w", name, err)
	}
	newProvider := c.newProvider
	if newProvider == nil {
		newProvider = (providerFactory{credentials: c.Credentials}).new
	}
	provider, err := newProvider(name, reference)
	if err != nil {
		return fmt.Errorf("create %s DNS control check: %s", name, redactDNSControlError(err, values))
	}
	random := c.Random
	if random == nil {
		random = rand.Reader
	}
	nonce := make([]byte, 32)
	if _, err := io.ReadFull(random, nonce); err != nil {
		return fmt.Errorf("create DNS control check nonce: %w", err)
	}
	token := "tlsferry-dns-check"
	keyAuth := token + "." + base64.RawURLEncoding.EncodeToString(nonce)
	if err := provider.Present(domain, token, keyAuth); err != nil {
		return fmt.Errorf("write temporary ACME challenge record for %q: %s", domain, redactDNSControlError(err, values))
	}
	if err := provider.CleanUp(domain, token, keyAuth); err != nil {
		return fmt.Errorf("temporary ACME challenge record for %q was created but cleanup failed: %s", domain, redactDNSControlError(err, values))
	}
	return nil
}

func directDNSProviderCredentialFields(name string) ([]string, bool) {
	switch name {
	case "cloudflare":
		return []string{"API_TOKEN"}, true
	case "dnspod":
		return []string{"SECRET_ID", "SECRET_KEY"}, true
	case "aliyun":
		return []string{"ACCESS_KEY_ID", "ACCESS_KEY_SECRET"}, true
	default:
		return nil, false
	}
}

func redactDNSControlError(err error, values map[string]string) string {
	message := err.Error()
	for _, value := range values {
		if value != "" {
			message = strings.ReplaceAll(message, value, "[REDACTED]")
		}
	}
	return message
}

func (f providerFactory) new(name, reference string) (challenge.Provider, error) {
	switch name {
	case "tlsferry-cloud":
		values, err := f.credentials.Values(reference, "API_URL", "API_TOKEN")
		if err != nil {
			return nil, fmt.Errorf("TLSFerry Cloud DNS credentials: %w", err)
		}
		provider, err := newRemoteDNSProvider(values["API_URL"], values["API_TOKEN"], nil)
		if err != nil {
			return nil, fmt.Errorf("create TLSFerry Cloud DNS provider: %w", err)
		}
		return provider, nil

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
