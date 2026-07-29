package service

import (
	"reflect"
	"testing"
)

func TestKickstartLaunchdDoesNotKillActiveRenewal(t *testing.T) {
	var arguments []string
	err := kickstartLaunchd(func(args ...string) ([]byte, error) {
		arguments = append(arguments, args...)
		return nil, nil
	})
	if err != nil {
		t.Fatalf("kickstartLaunchd() error = %v", err)
	}
	want := []string{"kickstart", launchdDomain() + "/" + LaunchdLabel}
	if !reflect.DeepEqual(arguments, want) {
		t.Fatalf("launchctl arguments = %q, want %q", arguments, want)
	}
}
