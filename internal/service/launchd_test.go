package service

import (
	"strings"
	"testing"
)

func TestLaunchdPlistRunsRenewAtLoginAndDaily(t *testing.T) {
	plist, err := RenderLaunchd(LaunchdConfig{
		Executable: "/Applications/TLSFerry/tlsferry",
		ConfigPath: "/Users/test/TLSFerry/config.json",
		StateDir:   "/Users/test/.tlsferry",
		OutputDir:  "/Users/test/.tlsferry/certificates",
		LogDir:     "/Users/test/.tlsferry/logs",
		Hour:       3,
		Minute:     17,
	})
	if err != nil {
		t.Fatalf("RenderLaunchd() error = %v", err)
	}
	text := string(plist)
	for _, expected := range []string{
		"<string>renew</string>",
		"<string>--accept-tos</string>",
		"<string>--execute</string>",
		"<key>RunAtLoad</key>",
		"<key>StartCalendarInterval</key>",
		"<integer>3</integer>",
		"<integer>17</integer>",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("plist does not contain %q:\n%s", expected, text)
		}
	}
}

func TestLaunchdPlistRejectsRelativePaths(t *testing.T) {
	_, err := RenderLaunchd(LaunchdConfig{Executable: "tlsferry", ConfigPath: "/tmp/config.json", StateDir: "/tmp/state", OutputDir: "/tmp/output", LogDir: "/tmp/logs"})
	if err == nil {
		t.Fatal("RenderLaunchd() succeeded with a relative executable")
	}
}
