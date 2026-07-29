package service

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"path/filepath"
)

const LaunchdLabel = "com.nonozone.tlsferry"

type LaunchdConfig struct {
	Executable string
	ConfigPath string
	StateDir   string
	OutputDir  string
	LogDir     string
	Hour       int
	Minute     int
}

func RenderLaunchd(config LaunchdConfig) ([]byte, error) {
	for name, path := range map[string]string{
		"executable": config.Executable,
		"config":     config.ConfigPath,
		"state":      config.StateDir,
		"output":     config.OutputDir,
		"log":        config.LogDir,
	} {
		if !filepath.IsAbs(path) {
			return nil, fmt.Errorf("%s path must be absolute", name)
		}
	}
	if config.Hour < 0 || config.Hour > 23 || config.Minute < 0 || config.Minute > 59 {
		return nil, fmt.Errorf("schedule must use a valid hour and minute")
	}

	args := []string{
		config.Executable,
		"renew",
		"--config", config.ConfigPath,
		"--state-dir", config.StateDir,
		"--output-dir", config.OutputDir,
		"--accept-tos",
		"--execute",
	}
	var output bytes.Buffer
	output.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	output.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n")
	output.WriteString("<plist version=\"1.0\">\n<dict>\n")
	writeKeyString(&output, "Label", LaunchdLabel)
	output.WriteString("  <key>ProgramArguments</key>\n  <array>\n")
	for _, argument := range args {
		output.WriteString("    <string>")
		if err := xml.EscapeText(&output, []byte(argument)); err != nil {
			return nil, err
		}
		output.WriteString("</string>\n")
	}
	output.WriteString("  </array>\n")
	output.WriteString("  <key>RunAtLoad</key>\n  <true/>\n")
	output.WriteString("  <key>StartCalendarInterval</key>\n  <dict>\n")
	fmt.Fprintf(&output, "    <key>Hour</key>\n    <integer>%d</integer>\n", config.Hour)
	fmt.Fprintf(&output, "    <key>Minute</key>\n    <integer>%d</integer>\n", config.Minute)
	output.WriteString("  </dict>\n")
	writeKeyString(&output, "StandardOutPath", filepath.Join(config.LogDir, "renew.log"))
	writeKeyString(&output, "StandardErrorPath", filepath.Join(config.LogDir, "renew.error.log"))
	output.WriteString("</dict>\n</plist>\n")
	return output.Bytes(), nil
}

func writeKeyString(output *bytes.Buffer, key, value string) {
	fmt.Fprintf(output, "  <key>%s</key>\n  <string>", key)
	_ = xml.EscapeText(output, []byte(value))
	output.WriteString("</string>\n")
}
