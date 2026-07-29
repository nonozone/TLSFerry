package discovery

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestManagerScansAndSortsCloudDomains(t *testing.T) {
	manager := Manager{Scanners: map[string]Scanner{
		"tencent": scannerFunc(func(context.Context) ([]Domain, error) {
			return []Domain{
				{Provider: "tencent", Name: "static.example.com", Status: "online", HTTPS: true},
				{Provider: "tencent", Name: "img.example.com", Status: "offline"},
			}, nil
		}),
	}}

	domains, err := manager.Scan(context.Background(), "tencent")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	want := []Domain{
		{Provider: "tencent", Name: "img.example.com", Status: "offline"},
		{Provider: "tencent", Name: "static.example.com", Status: "online", HTTPS: true},
	}
	if !reflect.DeepEqual(domains, want) {
		t.Fatalf("Scan() = %#v, want %#v", domains, want)
	}
}

func TestManagerRejectsUnknownProvider(t *testing.T) {
	_, err := (Manager{}).Scan(context.Background(), "unknown")
	if err == nil {
		t.Fatal("Scan() succeeded for an unknown provider")
	}
}

func TestManagerPreservesScannerError(t *testing.T) {
	want := errors.New("permission denied")
	manager := Manager{Scanners: map[string]Scanner{
		"aliyun": scannerFunc(func(context.Context) ([]Domain, error) { return nil, want }),
	}}
	_, err := manager.Scan(context.Background(), "aliyun")
	if !errors.Is(err, want) {
		t.Fatalf("Scan() error = %v, want wrapped %v", err, want)
	}
}

type scannerFunc func(context.Context) ([]Domain, error)

func (f scannerFunc) Scan(ctx context.Context) ([]Domain, error) { return f(ctx) }
