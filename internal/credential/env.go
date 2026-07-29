package credential

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var envProfilePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

type LookupEnv func(string) (string, bool)

type EnvResolver struct {
	Lookup LookupEnv
}

func (r EnvResolver) Require(reference string, fields ...string) error {
	_, err := r.Values(reference, fields...)
	return err
}

func (r EnvResolver) Values(reference string, fields ...string) (map[string]string, error) {
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
