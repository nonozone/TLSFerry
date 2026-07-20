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
	profile, err := ParseEnvReference(reference)
	if err != nil {
		return err
	}

	lookup := r.Lookup
	if lookup == nil {
		lookup = os.LookupEnv
	}

	var missing []string
	for _, field := range fields {
		name := profile + "_" + field
		value, ok := lookup(name)
		if !ok || strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing environment variable(s): %s", strings.Join(missing, ", "))
	}
	return nil
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
