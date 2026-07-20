package credential

import "testing"

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
