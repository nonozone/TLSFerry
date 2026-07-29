package service

import (
	"strings"
	"testing"
)

func TestSystemdUnitsRunRenewDailyAndPersistMissedRuns(t *testing.T) {
	serviceUnit, timerUnit, err := RenderSystemd(SystemdConfig{
		Executable: "/usr/local/bin/tlsferry",
		ConfigPath: "/home/test/.config/tlsferry/config.json",
		StateDir:   "/home/test/.local/state/tlsferry",
		OutputDir:  "/home/test/.local/state/tlsferry/certificates",
		Hour:       3,
		Minute:     17,
	})
	if err != nil {
		t.Fatalf("RenderSystemd() error = %v", err)
	}
	for _, expected := range []string{"renew", "--accept-tos", "--execute", "UMask=0077", "NoNewPrivileges=true"} {
		if !strings.Contains(string(serviceUnit), expected) {
			t.Fatalf("service unit does not contain %q:\n%s", expected, serviceUnit)
		}
	}
	for _, expected := range []string{"OnCalendar=*-*-* 03:17:00", "Persistent=true", "Unit=tlsferry-renew.service"} {
		if !strings.Contains(string(timerUnit), expected) {
			t.Fatalf("timer unit does not contain %q:\n%s", expected, timerUnit)
		}
	}
}

func TestSystemdUnitsRejectRelativePaths(t *testing.T) {
	_, _, err := RenderSystemd(SystemdConfig{Executable: "tlsferry", ConfigPath: "/tmp/config.json", StateDir: "/tmp/state", OutputDir: "/tmp/output"})
	if err == nil {
		t.Fatal("RenderSystemd() succeeded with a relative executable")
	}
}

func TestSystemdQuoteEscapesSpecifierCharacters(t *testing.T) {
	if got := systemdQuote("/tmp/100%/tlsferry"); got != `"/tmp/100%%/tlsferry"` {
		t.Fatalf("systemdQuote() = %q", got)
	}
}
