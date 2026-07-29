package credential

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var envProfilePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

type LookupEnv func(string) (string, bool)

type Store interface {
	Get(profile string) (map[string]string, error)
	Set(profile string, values map[string]string) error
	Delete(profile string) error
}

type Resolver struct {
	Lookup LookupEnv
	Store  Store
}

// EnvResolver remains as a compatibility alias for existing integrations.
type EnvResolver = Resolver

func (r Resolver) Require(reference string, fields ...string) error {
	_, err := r.Values(reference, fields...)
	return err
}

func (r Resolver) Values(reference string, fields ...string) (map[string]string, error) {
	if strings.HasPrefix(reference, "keychain:") {
		return r.keychainValues(reference, fields...)
	}
	return r.envValues(reference, fields...)
}

func (r Resolver) envValues(reference string, fields ...string) (map[string]string, error) {
	profile, err := ParseEnvReference(reference)
	if err != nil {
		return nil, err
	}

	lookup := r.Lookup
	if lookup == nil {
		lookup = os.LookupEnv
	}

	var missing []string
	values := make(map[string]string, len(fields))
	for _, field := range fields {
		name := profile + "_" + field
		value, ok := lookup(name)
		if !ok || strings.TrimSpace(value) == "" {
			missing = append(missing, name)
			continue
		}
		values[field] = value
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing environment variable(s): %s", strings.Join(missing, ", "))
	}
	return values, nil
}

func (r Resolver) keychainValues(reference string, fields ...string) (map[string]string, error) {
	profile, err := ParseKeychainReference(reference)
	if err != nil {
		return nil, err
	}
	store := r.Store
	if store == nil {
		store = KeyringStore{}
	}
	stored, err := store.Get(profile)
	if err != nil {
		return nil, fmt.Errorf("read keychain credential %s: %w", profile, err)
	}
	values := make(map[string]string, len(fields))
	var missing []string
	for _, field := range fields {
		value := strings.TrimSpace(stored[field])
		if value == "" {
			missing = append(missing, field)
			continue
		}
		values[field] = value
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing keychain credential field(s) for %s: %s", profile, strings.Join(missing, ", "))
	}
	return values, nil
}

func ParseEnvReference(reference string) (string, error) {
	const prefix = "env:"
	if !strings.HasPrefix(reference, prefix) {
		return "", fmt.Errorf("unsupported credential reference %q; expected env:PROFILE", reference)
	}
	profile := strings.TrimSpace(strings.TrimPrefix(reference, prefix))
	if !envProfilePattern.MatchString(profile) {
		return "", fmt.Errorf("invalid environment credential profile %q", profile)
	}
	return profile, nil
}

func ParseKeychainReference(reference string) (string, error) {
	const prefix = "keychain:"
	if !strings.HasPrefix(reference, prefix) {
		return "", fmt.Errorf("unsupported credential reference %q; expected keychain:PROFILE", reference)
	}
	profile := strings.TrimSpace(strings.TrimPrefix(reference, prefix))
	if !envProfilePattern.MatchString(profile) {
		return "", fmt.Errorf("invalid keychain credential profile %q", profile)
	}
	return profile, nil
}
