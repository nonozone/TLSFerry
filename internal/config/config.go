package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var certificateNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
var domainLabelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

type Config struct {
	RenewBefore  string        `json:"renew_before"`
	Certificates []Certificate `json:"certificates"`
}

type Certificate struct {
	Name        string       `json:"name"`
	Domains     []string     `json:"domains"`
	Issuer      Issuer       `json:"issuer"`
	Deployments []Deployment `json:"deployments"`
}

type Issuer struct {
	Type         string `json:"type"`
	Email        string `json:"email"`
	DirectoryURL string `json:"directory_url"`
	Challenge    string `json:"challenge"`
	DNSProvider  string `json:"dns_provider"`
	Credential   string `json:"credential"`
}

type Deployment struct {
	Provider   string            `json:"provider"`
	Target     string            `json:"target"`
	Credential string            `json:"credential"`
	Options    map[string]string `json:"options,omitempty"`
}

func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	var cfg Config
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return Config{}, fmt.Errorf("decode %s: trailing content: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", path, err)
	}
	encoded = append(encoded, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	temporary, err := os.CreateTemp(dir, ".tlsferry-config-*")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(encoded); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

func NormalizeDomain(value string) (string, error) {
	domain := strings.ToLower(strings.TrimSpace(value))
	domain = strings.TrimSuffix(domain, ".")
	if domain == "" || len(domain) > 253 || net.ParseIP(domain) != nil {
		return "", fmt.Errorf("invalid domain %q", value)
	}
	if strings.HasPrefix(domain, "*.") {
		domain = strings.TrimPrefix(domain, "*.")
		if domain == "" {
			return "", fmt.Errorf("invalid domain %q", value)
		}
	}
	for _, label := range strings.Split(domain, ".") {
		if !domainLabelPattern.MatchString(label) {
			return "", fmt.Errorf("invalid domain %q", value)
		}
	}
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.TrimSuffix(normalized, ".")
	return normalized, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.RenewBefore) == "" {
		return errors.New("renew_before is required")
	}
	renewBefore, err := time.ParseDuration(c.RenewBefore)
	if err != nil || renewBefore <= 0 {
		return fmt.Errorf("renew_before must be a positive Go duration: %q", c.RenewBefore)
	}
	if len(c.Certificates) == 0 {
		return errors.New("at least one certificate is required")
	}

	names := make(map[string]struct{}, len(c.Certificates))
	for i, certificate := range c.Certificates {
		if err := certificate.validate(); err != nil {
			return fmt.Errorf("certificates[%d]: %w", i, err)
		}
		if _, exists := names[certificate.Name]; exists {
			return fmt.Errorf("duplicate certificate name %q", certificate.Name)
		}
		names[certificate.Name] = struct{}{}
	}
	return nil
}

func (c Certificate) validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("name is required")
	}
	if !certificateNamePattern.MatchString(c.Name) {
		return errors.New("name may contain only letters, numbers, dots, underscores, and hyphens")
	}
	if len(c.Domains) == 0 {
		return errors.New("at least one domain is required")
	}
	domains := make(map[string]struct{}, len(c.Domains))
	for _, value := range c.Domains {
		domain, err := NormalizeDomain(value)
		if err != nil {
			return err
		}
		if _, exists := domains[domain]; exists {
			return fmt.Errorf("duplicate domain %q", domain)
		}
		domains[domain] = struct{}{}
	}

	if c.Issuer.Type != "acme" {
		return fmt.Errorf("unsupported issuer type %q", c.Issuer.Type)
	}
	if strings.TrimSpace(c.Issuer.Email) == "" {
		return errors.New("issuer.email is required")
	}
	directoryURL, err := url.ParseRequestURI(c.Issuer.DirectoryURL)
	if err != nil || directoryURL.Scheme != "https" || directoryURL.Host == "" {
		return errors.New("issuer.directory_url must be a valid HTTPS URL")
	}
	if c.Issuer.Challenge != "dns-01" {
		return fmt.Errorf("unsupported issuer challenge %q; the MVP supports dns-01", c.Issuer.Challenge)
	}
	if strings.TrimSpace(c.Issuer.DNSProvider) == "" {
		return errors.New("issuer.dns_provider is required")
	}
	if strings.TrimSpace(c.Issuer.Credential) == "" {
		return errors.New("issuer.credential is required")
	}
	for i, deployment := range c.Deployments {
		if strings.TrimSpace(deployment.Provider) == "" {
			return fmt.Errorf("deployments[%d].provider is required", i)
		}
		if strings.TrimSpace(deployment.Target) == "" {
			return fmt.Errorf("deployments[%d].target is required", i)
		}
		if strings.TrimSpace(deployment.Credential) == "" {
			return fmt.Errorf("deployments[%d].credential is required", i)
		}
	}
	return nil
}
