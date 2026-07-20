package engine

import (
	"fmt"
	"io"
	"strings"

	"github.com/nonozone/TLSFerry/internal/config"
)

type Plan struct {
	RenewBefore  string
	Certificates []CertificatePlan
}

type CertificatePlan struct {
	Name        string
	Domains     []string
	Issuer      config.Issuer
	Deployments []config.Deployment
}

func BuildPlan(cfg config.Config) Plan {
	plan := Plan{
		RenewBefore:  cfg.RenewBefore,
		Certificates: make([]CertificatePlan, 0, len(cfg.Certificates)),
	}
	for _, certificate := range cfg.Certificates {
		plan.Certificates = append(plan.Certificates, CertificatePlan{
			Name:        certificate.Name,
			Domains:     append([]string(nil), certificate.Domains...),
			Issuer:      certificate.Issuer,
			Deployments: append([]config.Deployment(nil), certificate.Deployments...),
		})
	}
	return plan
}

func (p Plan) Render(w io.Writer) {
	fmt.Fprintf(w, "TLSFerry plan (renew when validity is below %s)\n", p.RenewBefore)
	for _, certificate := range p.Certificates {
		fmt.Fprintf(w, "\n%s\n", certificate.Name)
		fmt.Fprintf(w, "  domains: %s\n", strings.Join(certificate.Domains, ", "))
		fmt.Fprintf(
			w,
			"  issue:   %s via %s using %s\n",
			certificate.Issuer.Type,
			certificate.Issuer.Challenge,
			certificate.Issuer.DNSProvider,
		)
		for _, deployment := range certificate.Deployments {
			fmt.Fprintf(w, "  deploy:  %s -> %s\n", deployment.Provider, deployment.Target)
		}
	}
}
