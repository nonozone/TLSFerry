package preflight

import (
	"fmt"
	"strings"

	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/credential"
)

type Checker struct {
	Credentials credential.EnvResolver
}

type providerRequirement struct {
	fields []string
}

var dnsProviders = map[string]providerRequirement{
	"tlsferry-cloud": {fields: []string{"API_URL", "API_TOKEN"}},
	"cloudflare":     {fields: []string{"API_TOKEN"}},
	"dnspod":         {fields: []string{"SECRET_ID", "SECRET_KEY"}},
	"aliyun":         {fields: []string{"ACCESS_KEY_ID", "ACCESS_KEY_SECRET"}},
}

var deploymentProviders = map[string]providerRequirement{
	"tencent-cdn": {fields: []string{"SECRET_ID", "SECRET_KEY"}},
	"tencent-cos": {fields: []string{"SECRET_ID", "SECRET_KEY"}},
	"aliyun-cdn":  {fields: []string{"ACCESS_KEY_ID", "ACCESS_KEY_SECRET"}},
	"qiniu-cdn":   {fields: []string{"ACCESS_KEY", "SECRET_KEY"}},
}

func (c Checker) Check(cfg config.Config) error {
	var problems []string
	for _, certificate := range cfg.Certificates {
		requirement, ok := dnsProviders[certificate.Issuer.DNSProvider]
		if !ok {
			problems = append(problems, fmt.Sprintf("certificate %q: unsupported DNS provider %q", certificate.Name, certificate.Issuer.DNSProvider))
		} else if err := c.Credentials.Require(certificate.Issuer.Credential, requirement.fields...); err != nil {
			problems = append(problems, fmt.Sprintf("certificate %q issuer: %v", certificate.Name, err))
		}

		for _, deployment := range certificate.Deployments {
			requirement, ok := deploymentProviders[deployment.Provider]
			if !ok {
				problems = append(problems, fmt.Sprintf("certificate %q: unsupported deployment provider %q", certificate.Name, deployment.Provider))
				continue
			}
			if err := c.Credentials.Require(deployment.Credential, requirement.fields...); err != nil {
				problems = append(problems, fmt.Sprintf("certificate %q deployment %q: %v", certificate.Name, deployment.Provider, err))
			}
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("preflight failed:\n- %s", strings.Join(problems, "\n- "))
	}
	return nil
}
