package certstore

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

var safeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type Bundle struct {
	Domains           []string
	Certificate       []byte
	IssuerCertificate []byte
	PrivateKey        []byte
	IssuedAt          time.Time
}

type Paths struct {
	Certificate string
	Issuer      string
	FullChain   string
	PrivateKey  string
	Metadata    string
}

type Store struct {
	Root string
}

type metadata struct {
	Name     string    `json:"name"`
	Domains  []string  `json:"domains"`
	IssuedAt time.Time `json:"issued_at"`
}

func (s Store) Save(name string, bundle Bundle) (Paths, error) {
	if !safeName.MatchString(name) {
		return Paths{}, fmt.Errorf("unsafe certificate name %q", name)
	}
	if len(bundle.Certificate) == 0 || len(bundle.PrivateKey) == 0 {
		return Paths{}, errors.New("certificate and private key are required")
	}
	if s.Root == "" {
		return Paths{}, errors.New("certificate output directory is required")
	}

	dir := filepath.Join(s.Root, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Paths{}, fmt.Errorf("create certificate directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return Paths{}, fmt.Errorf("secure certificate directory: %w", err)
	}

	paths := Paths{
		Certificate: filepath.Join(dir, "cert.pem"),
		Issuer:      filepath.Join(dir, "chain.pem"),
		FullChain:   filepath.Join(dir, "fullchain.pem"),
		PrivateKey:  filepath.Join(dir, "key.pem"),
		Metadata:    filepath.Join(dir, "metadata.json"),
	}
	metadata, err := json.MarshalIndent(metadata{Name: name, Domains: bundle.Domains, IssuedAt: bundle.IssuedAt}, "", "  ")
	if err != nil {
		return Paths{}, fmt.Errorf("encode certificate metadata: %w", err)
	}
	metadata = append(metadata, '\n')

	files := []struct {
		path string
		data []byte
		mode os.FileMode
	}{
		{paths.Certificate, bundle.Certificate, 0o644},
		{paths.Issuer, bundle.IssuerCertificate, 0o644},
		{paths.FullChain, append(append([]byte(nil), bundle.Certificate...), bundle.IssuerCertificate...), 0o644},
		{paths.PrivateKey, bundle.PrivateKey, 0o600},
		{paths.Metadata, metadata, 0o644},
	}
	for _, file := range files {
		if err := atomicWrite(file.path, file.data, file.mode); err != nil {
			return Paths{}, err
		}
	}
	return paths, nil
}

func (s Store) Load(name string) (Bundle, error) {
	if !safeName.MatchString(name) {
		return Bundle{}, fmt.Errorf("unsafe certificate name %q", name)
	}
	if s.Root == "" {
		return Bundle{}, errors.New("certificate input directory is required")
	}
	dir := filepath.Join(s.Root, name)
	certificate, err := os.ReadFile(filepath.Join(dir, "cert.pem"))
	if err != nil {
		return Bundle{}, fmt.Errorf("read certificate: %w", err)
	}
	issuerCertificate, err := os.ReadFile(filepath.Join(dir, "chain.pem"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Bundle{}, fmt.Errorf("read issuer certificate: %w", err)
	}
	privateKey, err := os.ReadFile(filepath.Join(dir, "key.pem"))
	if err != nil {
		return Bundle{}, fmt.Errorf("read private key: %w", err)
	}
	if _, err := tls.X509KeyPair(append(append([]byte(nil), certificate...), issuerCertificate...), privateKey); err != nil {
		return Bundle{}, fmt.Errorf("validate stored certificate and private key: %w", err)
	}

	metadataBytes, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		return Bundle{}, fmt.Errorf("read certificate metadata: %w", err)
	}
	var saved metadata
	if err := json.Unmarshal(metadataBytes, &saved); err != nil {
		return Bundle{}, fmt.Errorf("decode certificate metadata: %w", err)
	}
	if saved.Name != name {
		return Bundle{}, errors.New("certificate metadata name does not match its directory")
	}
	return Bundle{
		Domains:           saved.Domains,
		Certificate:       certificate,
		IssuerCertificate: issuerCertificate,
		PrivateKey:        privateKey,
		IssuedAt:          saved.IssuedAt,
	}, nil
}

func (b Bundle) FullChain() []byte {
	return append(append([]byte(nil), b.Certificate...), b.IssuerCertificate...)
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".tlsferry-*")
	if err != nil {
		return fmt.Errorf("create temporary file for %s: %w", path, err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return fmt.Errorf("set permissions for %s: %w", path, err)
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync %s: %w", path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
