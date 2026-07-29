package service

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode/utf16"
)

const WindowsTaskName = "TLSFerry Renewal"

var windowsDrivePath = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

type WindowsTaskConfig struct {
	UserID     string
	Executable string
	ConfigPath string
	StateDir   string
	OutputDir  string
	Hour       int
	Minute     int
}

func RenderWindowsTask(config WindowsTaskConfig) ([]byte, error) {
	if strings.TrimSpace(config.UserID) == "" {
		return nil, fmt.Errorf("Windows task user ID is required")
	}
	for name, path := range map[string]string{
		"executable": config.Executable,
		"config":     config.ConfigPath,
		"state":      config.StateDir,
		"output":     config.OutputDir,
	} {
		if !isWindowsAbsolutePath(path) {
			return nil, fmt.Errorf("%s path must be an absolute Windows path", name)
		}
	}
	if config.Hour < 0 || config.Hour > 23 || config.Minute < 0 || config.Minute > 59 {
		return nil, fmt.Errorf("schedule must use a valid hour and minute")
	}

	arguments := []string{
		"renew",
		"--config", config.ConfigPath,
		"--state-dir", config.StateDir,
		"--output-dir", config.OutputDir,
		"--accept-tos",
		"--execute",
	}
	quoted := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		quoted = append(quoted, windowsQuoteArgument(argument))
	}

	var output bytes.Buffer
	output.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	output.WriteString("<Task version=\"1.4\" xmlns=\"http://schemas.microsoft.com/windows/2004/02/mit/task\">\n")
	output.WriteString("  <RegistrationInfo><Description>TLSFerry certificate renewal</Description></RegistrationInfo>\n")
	output.WriteString("  <Triggers><CalendarTrigger>\n")
	fmt.Fprintf(&output, "    <StartBoundary>2000-01-01T%02d:%02d:00</StartBoundary>\n", config.Hour, config.Minute)
	output.WriteString("    <Enabled>true</Enabled><ScheduleByDay><DaysInterval>1</DaysInterval></ScheduleByDay>\n")
	output.WriteString("  </CalendarTrigger></Triggers>\n")
	output.WriteString("  <Principals><Principal id=\"Author\"><UserId>")
	writeWindowsXMLText(&output, config.UserID)
	output.WriteString("</UserId><LogonType>InteractiveToken</LogonType><RunLevel>LeastPrivilege</RunLevel></Principal></Principals>\n")
	output.WriteString("  <Settings>\n")
	output.WriteString("    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>\n")
	output.WriteString("    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>\n")
	output.WriteString("    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>\n")
	output.WriteString("    <StartWhenAvailable>true</StartWhenAvailable>\n")
	output.WriteString("    <RunOnlyIfNetworkAvailable>true</RunOnlyIfNetworkAvailable>\n")
	output.WriteString("    <ExecutionTimeLimit>PT1H</ExecutionTimeLimit><Enabled>true</Enabled>\n")
	output.WriteString("  </Settings>\n")
	output.WriteString("  <Actions Context=\"Author\"><Exec><Command>")
	writeWindowsXMLText(&output, config.Executable)
	output.WriteString("</Command><Arguments>")
	writeWindowsXMLText(&output, strings.Join(quoted, " "))
	output.WriteString("</Arguments></Exec></Actions>\n</Task>\n")
	return output.Bytes(), nil
}

func InstallWindowsTask(config WindowsTaskConfig) (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("Task Scheduler installation is supported on Windows only")
	}
	if strings.TrimSpace(config.UserID) == "" {
		current, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("resolve Windows task user: %w", err)
		}
		config.UserID = current.Username
	}
	task, err := RenderWindowsTask(config)
	if err != nil {
		return "", err
	}
	path, err := WindowsTaskPath()
	if err != nil {
		return "", err
	}
	for _, dir := range []string{filepath.Dir(path), config.StateDir, config.OutputDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", err
		}
	}
	if err := writeWindowsTaskFile(path, task); err != nil {
		return "", err
	}
	if err := runSchtasks("/Create", "/TN", WindowsTaskName, "/XML", path, "/F"); err != nil {
		return path, err
	}
	return path, nil
}

func UninstallWindowsTask() (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("Task Scheduler installation is supported on Windows only")
	}
	path, err := WindowsTaskPath()
	if err != nil {
		return "", err
	}
	_ = runSchtasks("/Delete", "/TN", WindowsTaskName, "/F")
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return path, err
	}
	return path, nil
}

func KickstartWindowsTask() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("Task Scheduler installation is supported on Windows only")
	}
	return runSchtasks("/Run", "/TN", WindowsTaskName)
}

func WindowsTaskStatus() (bool, string, error) {
	path, err := WindowsTaskPath()
	if err != nil {
		return false, "", err
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, path, nil
		}
		return false, path, err
	}
	if runtime.GOOS != "windows" {
		return false, path, nil
	}
	err = runSchtasks("/Query", "/TN", WindowsTaskName)
	return err == nil, path, nil
}

func WindowsTaskPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "TLSFerry", "tlsferry-renew-task.xml"), nil
}

func WindowsTaskLogsCommand() string {
	return `schtasks.exe /Query /TN "TLSFerry Renewal" /V /FO LIST`
}

func isWindowsAbsolutePath(path string) bool {
	return windowsDrivePath.MatchString(path) || strings.HasPrefix(path, `\\`)
}

func windowsQuoteArgument(value string) string {
	if value != "" && !strings.ContainsAny(value, " \t\"") {
		return value
	}
	var output strings.Builder
	output.WriteByte('"')
	backslashes := 0
	for _, char := range value {
		if char == '\\' {
			backslashes++
			continue
		}
		if char == '"' {
			output.WriteString(strings.Repeat("\\", backslashes*2+1))
			output.WriteRune(char)
			backslashes = 0
			continue
		}
		output.WriteString(strings.Repeat("\\", backslashes))
		backslashes = 0
		output.WriteRune(char)
	}
	output.WriteString(strings.Repeat("\\", backslashes*2))
	output.WriteByte('"')
	return output.String()
}

func writeWindowsXMLText(output *bytes.Buffer, value string) {
	_ = xml.EscapeText(output, []byte(value))
}

func writeWindowsTaskFile(path string, content []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".tlsferry-task-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(encodeWindowsTaskFile(content)); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func encodeWindowsTaskFile(content []byte) []byte {
	text := strings.Replace(string(content), `encoding="UTF-8"`, `encoding="UTF-16"`, 1)
	units := utf16.Encode([]rune(text))
	encoded := make([]byte, 2, 2+len(units)*2)
	encoded[0], encoded[1] = 0xff, 0xfe
	for _, unit := range units {
		encoded = append(encoded, byte(unit), byte(unit>>8))
	}
	return encoded
}

func runSchtasks(arguments ...string) error {
	output, err := exec.Command("schtasks.exe", arguments...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks.exe %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}
