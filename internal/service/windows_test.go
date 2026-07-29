package service

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestWindowsTaskRunsRenewDailyAndStartsWhenAvailable(t *testing.T) {
	task, err := RenderWindowsTask(WindowsTaskConfig{
		UserID:     `DESKTOP\test`,
		Executable: `C:\Program Files\TLSFerry\tlsferry.exe`,
		ConfigPath: `C:\Users\test\AppData\Roaming\TLSFerry\config.json`,
		StateDir:   `C:\Users\test\AppData\Local\TLSFerry`,
		OutputDir:  `C:\Users\test\AppData\Local\TLSFerry\certificates`,
		Hour:       3,
		Minute:     17,
	})
	if err != nil {
		t.Fatalf("RenderWindowsTask() error = %v", err)
	}
	if err := xml.Unmarshal(task, &struct{}{}); err != nil {
		t.Fatalf("task XML is invalid: %v", err)
	}
	text := string(task)
	for _, expected := range []string{
		`<StartBoundary>2000-01-01T03:17:00</StartBoundary>`,
		`<DaysInterval>1</DaysInterval>`,
		`<StartWhenAvailable>true</StartWhenAvailable>`,
		`<MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>`,
		`<RunOnlyIfNetworkAvailable>true</RunOnlyIfNetworkAvailable>`,
		`<Command>C:\Program Files\TLSFerry\tlsferry.exe</Command>`,
		`--accept-tos`,
		`--execute`,
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("task does not contain %q:\n%s", expected, text)
		}
	}
}

func TestWindowsTaskRejectsRelativePaths(t *testing.T) {
	_, err := RenderWindowsTask(WindowsTaskConfig{
		Executable: "tlsferry.exe", ConfigPath: `C:\config.json`, StateDir: `C:\state`, OutputDir: `C:\output`,
	})
	if err == nil {
		t.Fatal("RenderWindowsTask() succeeded with a relative executable")
	}
}

func TestWindowsQuoteArgumentPreservesSpacesAndQuotes(t *testing.T) {
	if got := windowsQuoteArgument(`C:\Users\A B\config.json`); got != `"C:\Users\A B\config.json"` {
		t.Fatalf("windowsQuoteArgument() = %q", got)
	}
	if got := windowsQuoteArgument(`value"quoted`); got != `"value\"quoted"` {
		t.Fatalf("windowsQuoteArgument() with quote = %q", got)
	}
}
