package acmeissuer

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"github.com/nonozone/TLSFerry/internal/certstore"
	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/credential"
)

type Client struct {
	StateDir    string
	Credentials credential.EnvResolver
	Now         func() time.Time
}

func (c Client) Obtain(certificateConfig config.Certificate, acceptTerms bool) (certstore.Bundle, error) {
	if !acceptTerms {
		return certstore.Bundle{}, errors.New("ACME terms of service must be accepted explicitly")
	}
	if c.StateDir == "" {
		return certstore.Bundle{}, errors.New("ACME state directory is required")
	}

	provider, err := (providerFactory{credentials: c.Credentials}).new(
		certificateConfig.Issuer.DNSProvider,
		certificateConfig.Issuer.Credential,
	)
	if err != nil {
		return certstore.Bundle{}, err
	}

	store := accountStore{root: c.StateDir}
	user, _, err := store.loadOrCreate(certificateConfig.Issuer.Email, certificateConfig.Issuer.DirectoryURL)
	if err != nil {
		return certstore.Bundle{}, err
	}

	clientConfig := lego.NewConfig(user)
	clientConfig.CADirURL = certificateConfig.Issuer.DirectoryURL
	clientConfig.Certificate.KeyType = certcrypto.EC256
	client, err := lego.NewClient(clientConfig)
	if err != nil {
		return certstore.Bundle{}, fmt.Errorf("create ACME client: %w", err)
	}

	if err := client.Challenge.SetDNS01Provider(provider); err != nil {
		return certstore.Bundle{}, fmt.Errorf("configure DNS-01 challenge: %w", err)
	}

	if user.registration == nil {
		registrationResource, err := client.Registration.Register(registration.RegisterOptions{
			TermsOfServiceAgreed: true,
		})
		if err != nil {
			return certstore.Bundle{}, fmt.Errorf("register ACME account: %w", err)
		}
		user.registration = registrationResource
		if err := store.save(user); err != nil {
			return certstore.Bundle{}, err
		}
	}

	resource, err := client.Certificate.Obtain(certificate.ObtainRequest{
		Domains:                        certificateConfig.Domains,
		Bundle:                         false,
		AlwaysDeactivateAuthorizations: true,
	})
	if err != nil {
		return certstore.Bundle{}, fmt.Errorf("obtain ACME certificate: %w", err)
	}
	now := c.Now
	if now == nil {
		now = time.Now
	}
	return certstore.Bundle{
		Domains:           append([]string(nil), certificateConfig.Domains...),
		Certificate:       resource.Certificate,
		IssuerCertificate: resource.IssuerCertificate,
		PrivateKey:        resource.PrivateKey,
		IssuedAt:          now().UTC(),
	}, nil
}
