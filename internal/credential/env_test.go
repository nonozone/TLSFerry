package credential

import (
	"reflect"
	"testing"
)

func TestEnvResolverRequire(t *testing.T) {
	values := map[string]string{
		"TENCENTCLOUD_SECRET_ID":  "id",
		"TENCENTCLOUD_SECRET_KEY": "key",
	}
	resolver := EnvResolver{Lookup: func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}}

	if err := resolver.Require("env:TENCENTCLOUD", "SECRET_ID", "SECRET_KEY"); err != nil {
		t.Fatalf("Require() returned an unexpected error: %v", err)
	}
}

func TestEnvResolverReportsMissingVariable(t *testing.T) {
	resolver := EnvResolver{Lookup: func(string) (string, bool) { return "", false }}
	if err := resolver.Require("env:ALIYUN", "ACCESS_KEY_ID"); err == nil {
		t.Fatal("Require() succeeded with a missing environment variable")
	}
}

func TestParseEnvReferenceRejectsInvalidProfile(t *testing.T) {
	if _, err := ParseEnvReference("env:tencent-cloud"); err == nil {
		t.Fatal("ParseEnvReference() succeeded for an invalid profile")
	}
}

func TestEnvResolverValues(t *testing.T) {
	resolver := EnvResolver{Lookup: func(name string) (string, bool) {
		values := map[string]string{"ALIYUN_ACCESS_KEY_ID": "id", "ALIYUN_ACCESS_KEY_SECRET": "secret"}
		value, ok := values[name]
		return value, ok
	}}

	values, err := resolver.Values("env:ALIYUN", "ACCESS_KEY_ID", "ACCESS_KEY_SECRET")
	if err != nil {
		t.Fatalf("Values() returned an unexpected error: %v", err)
	}
	if values["ACCESS_KEY_ID"] != "id" || values["ACCESS_KEY_SECRET"] != "secret" {
		t.Fatalf("Values() = %#v", values)
	}
}

func TestResolverReadsKeychainProfile(t *testing.T) {
	store := &memoryStore{values: map[string]map[string]string{
		"TENCENTCLOUD": {
			"SECRET_ID":  "id",
			"SECRET_KEY": "secret",
		},
	}}
	resolver := Resolver{Store: store}

	values, err := resolver.Values("keychain:TENCENTCLOUD", "SECRET_ID", "SECRET_KEY")
	if err != nil {
		t.Fatalf("Values() error = %v", err)
	}
	want := map[string]string{"SECRET_ID": "id", "SECRET_KEY": "secret"}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("Values() = %#v, want %#v", values, want)
	}
}

func TestResolverReportsMissingKeychainField(t *testing.T) {
	resolver := Resolver{Store: &memoryStore{values: map[string]map[string]string{
		"ALIYUN": {"ACCESS_KEY_ID": "id"},
	}}}

	_, err := resolver.Values("keychain:ALIYUN", "ACCESS_KEY_ID", "ACCESS_KEY_SECRET")
	if err == nil || err.Error() != "missing keychain credential field(s) for ALIYUN: ACCESS_KEY_SECRET" {
		t.Fatalf("Values() error = %v", err)
	}
}

type memoryStore struct {
	values map[string]map[string]string
}

func (s *memoryStore) Get(profile string) (map[string]string, error) {
	return s.values[profile], nil
}

func (s *memoryStore) Set(profile string, values map[string]string) error {
	s.values[profile] = values
	return nil
}

func (s *memoryStore) Delete(profile string) error {
	delete(s.values, profile)
	return nil
}
