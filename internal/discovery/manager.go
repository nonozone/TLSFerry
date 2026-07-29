package discovery

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type Domain struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	HTTPS    bool   `json:"https"`
	CNAME    string `json:"cname,omitempty"`
}

type Scanner interface {
	Scan(context.Context) ([]Domain, error)
}

type Manager struct {
	Scanners map[string]Scanner
}

func (m Manager) Scan(ctx context.Context, provider string) ([]Domain, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	scanner, ok := m.Scanners[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported discovery provider %q", provider)
	}
	domains, err := scanner.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("scan %s CDN domains: %w", provider, err)
	}
	sort.Slice(domains, func(i, j int) bool {
		return domains[i].Name < domains[j].Name
	})
	return domains, nil
}
