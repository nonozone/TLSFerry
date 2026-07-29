package acmeissuer

import (
	"crypto"
	"testing"

	"github.com/go-acme/lego/v4/registration"
)

func TestAccountStorePersistsAccount(t *testing.T) {
	store := accountStore{root: t.TempDir()}
	first, existed, err := store.loadOrCreate("ops@example.com", "https://acme.example/directory")
	if err != nil {
		t.Fatalf("loadOrCreate() returned an unexpected error: %v", err)
	}
	if existed {
		t.Fatal("new account was reported as existing")
	}
	first.registration = &registration.Resource{URI: "https://acme.example/account/1"}
	if err := store.save(first); err != nil {
		t.Fatalf("save() returned an unexpected error: %v", err)
	}

	second, existed, err := store.loadOrCreate("ops@example.com", "https://acme.example/directory")
	if err != nil {
		t.Fatalf("second loadOrCreate() returned an unexpected error: %v", err)
	}
	if !existed || second.registration == nil || second.registration.URI != first.registration.URI {
		t.Fatalf("persisted registration was not restored: %#v", second.registration)
	}
	assertSamePublicKey(t, first.privateKey, second.privateKey)
}

func assertSamePublicKey(t *testing.T, first, second crypto.PrivateKey) {
	t.Helper()
	firstPublic, ok := first.(interface{ Public() crypto.PublicKey })
	if !ok {
		t.Fatalf("first key type %T has no Public method", first)
	}
	secondPublic, ok := second.(interface{ Public() crypto.PublicKey })
	if !ok {
		t.Fatalf("second key type %T has no Public method", second)
	}
	if firstPublic.Public().(interface{ Equal(crypto.PublicKey) bool }).Equal(secondPublic.Public()) == false {
		t.Fatal("restored account key differs from the original")
	}
}
