package deployment

import (
	"context"
	"strings"
	"testing"

	"github.com/nonozone/TLSFerry/internal/certstore"
	"github.com/nonozone/TLSFerry/internal/config"
)

type fakeProvider struct {
	result Result
	err    error
}

func (f fakeProvider) Deploy(context.Context, string, config.Deployment, certstore.Bundle) (Result, error) {
	return f.result, f.err
}

func TestManagerDeploy(t *testing.T) {
	manager := Manager{providers: map[string]Provider{
		"tencent-cdn": fakeProvider{result: Result{Provider: "tencent-cdn", Target: "assets.example.com", Reference: "42", Status: "submitted"}},
	}}
	result, err := manager.Deploy(context.Background(), "assets", config.Deployment{Provider: "tencent-cdn", Target: "assets.example.com"}, certstore.Bundle{})
	if err != nil {
		t.Fatalf("Deploy() returned an unexpected error: %v", err)
	}
	if result.Reference != "42" || result.Status != "submitted" {
		t.Fatalf("Deploy() result = %#v", result)
	}
}

func TestManagerRejectsUnsupportedProvider(t *testing.T) {
	_, err := (Manager{}).Deploy(context.Background(), "assets", config.Deployment{Provider: "unknown"}, certstore.Bundle{})
	if err == nil || !strings.Contains(err.Error(), "unsupported deployment provider") {
		t.Fatalf("Deploy() error = %v", err)
	}
}
