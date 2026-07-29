package deployment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nonozone/TLSFerry/internal/certstore"
	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/credential"
)

func TestQiniuDeploysToCDN(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "QBox ") {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		paths = append(paths, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/sslcert":
			json.NewEncoder(w).Encode(map[string]string{"certID": "cert-1"})
		case r.Method == http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{"https": map[string]any{"certId": "old", "forceHttps": true, "http2Enable": true}})
		case r.Method == http.MethodPut:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	provider := qiniuProvider{
		credentials: credential.EnvResolver{Lookup: credentialLookup(map[string]string{
			"QINIU_ACCESS_KEY": "access", "QINIU_SECRET_KEY": "secret",
		})},
		baseURL: server.URL,
		client:  server.Client(),
	}
	result, err := provider.Deploy(context.Background(), "assets", config.Deployment{
		Provider: "qiniu-cdn", Target: "assets.example.com", Credential: "env:QINIU",
	}, certstore.Bundle{Certificate: []byte("cert"), PrivateKey: []byte("key")})
	if err != nil {
		t.Fatalf("Deploy() returned an unexpected error: %v", err)
	}
	if result.Reference != "cert-1" || len(paths) != 3 || paths[2] != "PUT /domain/assets.example.com/httpsconf" {
		t.Fatalf("result = %#v, paths = %#v", result, paths)
	}
}
