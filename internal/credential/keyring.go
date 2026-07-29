package credential

import (
	"encoding/json"
	"errors"
	"fmt"

	keyring "github.com/zalando/go-keyring"
)

const keyringService = "TLSFerry"

type KeyringStore struct{}

func (KeyringStore) Get(profile string) (map[string]string, error) {
	encoded, err := keyring.Get(keyringService, profile)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, fmt.Errorf("profile %s was not found; run tlsferry auth login first", profile)
	}
	if err != nil {
		return nil, err
	}
	var values map[string]string
	if err := json.Unmarshal([]byte(encoded), &values); err != nil {
		return nil, fmt.Errorf("decode stored profile: %w", err)
	}
	return values, nil
}

func (KeyringStore) Set(profile string, values map[string]string) error {
	if !envProfilePattern.MatchString(profile) {
		return fmt.Errorf("invalid keychain credential profile %q", profile)
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return err
	}
	return keyring.Set(keyringService, profile, string(encoded))
}

func (KeyringStore) Delete(profile string) error {
	err := keyring.Delete(keyringService, profile)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
