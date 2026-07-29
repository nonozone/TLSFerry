package acmeissuer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-acme/lego/v4/challenge/dns01"
)

func TestRemoteDNSProviderPresentsAndCleansChallenge(t *testing.T) {
	type requestRecord struct {
		Method        string
		Path          string
		Authorization string
		Body          remoteChallengeRequest
	}
	var requests []requestRecord
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record := requestRecord{
			Method:        r.Method,
			Path:          r.URL.Path,
			Authorization: r.Header.Get("Authorization"),
		}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&record.Body)
		}
		requests = append(requests, record)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	provider, err := newRemoteDNSProvider(server.URL, "job-token", server.Client())
	if err != nil {
		t.Fatalf("newRemoteDNSProvider() error = %v", err)
	}
	provider.challengeInfo = func(domain, keyAuth string) dns01.ChallengeInfo {
		return dns01.ChallengeInfo{
			FQDN:          "_acme-challenge.nos.nonopen.com.",
			EffectiveFQDN: "domain-123.auth.tlsferry-dcv.com.",
			Value:         "txt-value",
		}
	}

	if err := provider.Present("nos.nonopen.com", "acme-token", "key-auth"); err != nil {
		t.Fatalf("Present() error = %v", err)
	}
	if err := provider.CleanUp("nos.nonopen.com", "acme-token", "key-auth"); err != nil {
		t.Fatalf("CleanUp() error = %v", err)
	}

	if len(requests) != 2 {
		t.Fatalf("requests = %#v", requests)
	}
	if requests[0].Method != http.MethodPut || requests[1].Method != http.MethodDelete {
		t.Fatalf("methods = %s, %s", requests[0].Method, requests[1].Method)
	}
	if requests[0].Path == "/v1/acme-challenges/" || requests[0].Path != requests[1].Path {
		t.Fatalf("paths = %q, %q", requests[0].Path, requests[1].Path)
	}
	if requests[0].Authorization != "Bearer job-token" || requests[1].Authorization != "Bearer job-token" {
		t.Fatalf("authorization headers = %q, %q", requests[0].Authorization, requests[1].Authorization)
	}
	wantBody := remoteChallengeRequest{
		Domain:        "nos.nonopen.com",
		FQDN:          "_acme-challenge.nos.nonopen.com.",
		EffectiveFQDN: "domain-123.auth.tlsferry-dcv.com.",
		Value:         "txt-value",
	}
	if requests[0].Body != wantBody {
		t.Fatalf("request body = %#v, want %#v", requests[0].Body, wantBody)
	}
}

func TestRemoteDNSProviderRejectsInsecureRemoteURL(t *testing.T) {
	_, err := newRemoteDNSProvider("http://api.example.com", "job-token", nil)
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("newRemoteDNSProvider() error = %v", err)
	}
}

func TestRemoteDNSProviderReportsControlPlaneError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "domain is outside the job scope", http.StatusForbidden)
	}))
	t.Cleanup(server.Close)

	provider, err := newRemoteDNSProvider(server.URL, "job-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	provider.challengeInfo = func(domain, keyAuth string) dns01.ChallengeInfo {
		return dns01.ChallengeInfo{FQDN: "_acme-challenge.example.com.", EffectiveFQDN: "id.auth.example.net.", Value: "value"}
	}

	err = provider.Present("example.com", "token", "key-auth")
	if err == nil || !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "outside the job scope") {
		t.Fatalf("Present() error = %v", err)
	}
}

func TestRemoteDNSProviderTreatsMissingCleanupAsSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s", r.Method)
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(server.Close)

	provider, err := newRemoteDNSProvider(server.URL, "job-token", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	provider.challengeInfo = func(domain, keyAuth string) dns01.ChallengeInfo {
		return dns01.ChallengeInfo{FQDN: "_acme-challenge.example.com."}
	}

	if err := provider.CleanUp("example.com", "token", "key-auth"); err != nil {
		t.Fatalf("CleanUp() error = %v", err)
	}
}
