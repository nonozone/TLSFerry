package acmeissuer

import (
	"crypto"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/registration"
)

type account struct {
	email        string
	directoryURL string
	privateKey   crypto.PrivateKey
	registration *registration.Resource
}

func (a *account) GetEmail() string                        { return a.email }
func (a *account) GetRegistration() *registration.Resource { return a.registration }
func (a *account) GetPrivateKey() crypto.PrivateKey        { return a.privateKey }

type accountStore struct {
	root string
}

type persistedAccount struct {
	Email        string                 `json:"email"`
	DirectoryURL string                 `json:"directory_url"`
	Registration *registration.Resource `json:"registration,omitempty"`
}

func (s accountStore) loadOrCreate(email, directoryURL string) (*account, bool, error) {
	dir := s.directory(email, directoryURL)
	metadataPath := filepath.Join(dir, "account.json")
	keyPath := filepath.Join(dir, "account.key")
	metadata, metadataErr := os.ReadFile(metadataPath)
	keyPEM, keyErr := os.ReadFile(keyPath)
	if errors.Is(metadataErr, os.ErrNotExist) && errors.Is(keyErr, os.ErrNotExist) {
		privateKey, err := certcrypto.GeneratePrivateKey(certcrypto.EC256)
		if err != nil {
			return nil, false, fmt.Errorf("generate ACME account key: %w", err)
		}
		created := &account{email: email, directoryURL: directoryURL, privateKey: privateKey}
		if err := s.save(created); err != nil {
			return nil, false, err
		}
		return created, false, nil
	}
	if metadataErr != nil {
		return nil, false, fmt.Errorf("read ACME account metadata: %w", metadataErr)
	}
	if keyErr != nil {
		return nil, false, fmt.Errorf("read ACME account key: %w", keyErr)
	}

	var persisted persistedAccount
	if err := json.Unmarshal(metadata, &persisted); err != nil {
		return nil, false, fmt.Errorf("decode ACME account metadata: %w", err)
	}
	if persisted.Email != email || persisted.DirectoryURL != directoryURL {
		return nil, false, errors.New("ACME account metadata does not match the requested account")
	}
	privateKey, err := certcrypto.ParsePEMPrivateKey(keyPEM)
	if err != nil {
		return nil, false, fmt.Errorf("decode ACME account key: %w", err)
	}
	return &account{email: email, directoryURL: directoryURL, privateKey: privateKey, registration: persisted.Registration}, true, nil
}

func (s accountStore) save(value *account) error {
	dir := s.directory(value.email, value.directoryURL)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create ACME account directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("secure ACME account directory: %w", err)
	}
	metadata, err := json.MarshalIndent(persistedAccount{
		Email: value.email, DirectoryURL: value.directoryURL, Registration: value.registration,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode ACME account metadata: %w", err)
	}
	metadata = append(metadata, '\n')
	if err := writeFile(filepath.Join(dir, "account.json"), metadata, 0o600); err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, "account.key"), certcrypto.PEMEncode(value.privateKey), 0o600)
}

func (s accountStore) directory(email, directoryURL string) string {
	digest := sha256.Sum256([]byte(directoryURL + "\x00" + email))
	return filepath.Join(s.root, "accounts", hex.EncodeToString(digest[:16]))
}

func writeFile(path string, data []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".tlsferry-*")
	if err != nil {
		return fmt.Errorf("create temporary account file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
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
		return fmt.Errorf("replace account file: %w", err)
	}
	return nil
}
