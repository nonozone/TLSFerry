package service

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	SystemdServiceName = "tlsferry-renew.service"
	SystemdTimerName   = "tlsferry-renew.timer"
)

type SystemdConfig struct {
	Executable string
	ConfigPath string
	StateDir   string
	OutputDir  string
	Hour       int
	Minute     int
}

func RenderSystemd(config SystemdConfig) ([]byte, []byte, error) {
	for name, path := range map[string]string{
		"executable": config.Executable,
		"config":     config.ConfigPath,
		"state":      config.StateDir,
		"output":     config.OutputDir,
	} {
		if !filepath.IsAbs(path) {
			return nil, nil, fmt.Errorf("%s path must be absolute", name)
		}
	}
	if config.Hour < 0 || config.Hour > 23 || config.Minute < 0 || config.Minute > 59 {
		return nil, nil, fmt.Errorf("schedule must use a valid hour and minute")
	}

	arguments := []string{
		config.Executable,
		"renew",
		"--config", config.ConfigPath,
		"--state-dir", config.StateDir,
		"--output-dir", config.OutputDir,
		"--accept-tos",
		"--execute",
	}
	quoted := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		quoted = append(quoted, systemdQuote(argument))
	}

	var service bytes.Buffer
	service.WriteString("[Unit]\n")
	service.WriteString("Description=TLSFerry certificate renewal\n")
	service.WriteString("Wants=network-online.target\n")
	service.WriteString("After=network-online.target\n\n")
	service.WriteString("[Service]\n")
	service.WriteString("Type=oneshot\n")
	fmt.Fprintf(&service, "ExecStart=%s\n", strings.Join(quoted, " "))
	service.WriteString("UMask=0077\n")
	service.WriteString("NoNewPrivileges=true\n")
	service.WriteString("PrivateTmp=true\n")

	var timer bytes.Buffer
	timer.WriteString("[Unit]\n")
	timer.WriteString("Description=Run TLSFerry certificate renewal daily\n\n")
	timer.WriteString("[Timer]\n")
	fmt.Fprintf(&timer, "OnCalendar=*-*-* %02d:%02d:00\n", config.Hour, config.Minute)
	timer.WriteString("Persistent=true\n")
	timer.WriteString("Unit=" + SystemdServiceName + "\n\n")
	timer.WriteString("[Install]\n")
	timer.WriteString("WantedBy=timers.target\n")
	return service.Bytes(), timer.Bytes(), nil
}

func InstallSystemd(config SystemdConfig) (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("systemd service installation is supported on Linux only")
	}
	serviceUnit, timerUnit, err := RenderSystemd(config)
	if err != nil {
		return "", err
	}
	servicePath, timerPath, err := SystemdPaths()
	if err != nil {
		return "", err
	}
	for _, path := range []string{config.StateDir, config.OutputDir} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return "", err
		}
	}
	if err := os.MkdirAll(filepath.Dir(servicePath), 0o755); err != nil {
		return "", err
	}
	if err := writeSystemdFile(servicePath, serviceUnit); err != nil {
		return "", err
	}
	if err := writeSystemdFile(timerPath, timerUnit); err != nil {
		return "", err
	}
	if err := runSystemctl("daemon-reload"); err != nil {
		return timerPath, err
	}
	if err := runSystemctl("enable", "--now", SystemdTimerName); err != nil {
		return timerPath, err
	}
	return timerPath, nil
}

func UninstallSystemd() (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("systemd service installation is supported on Linux only")
	}
	servicePath, timerPath, err := SystemdPaths()
	if err != nil {
		return "", err
	}
	_ = runSystemctl("disable", "--now", SystemdTimerName)
	for _, path := range []string{servicePath, timerPath} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return timerPath, err
		}
	}
	if err := runSystemctl("daemon-reload"); err != nil {
		return timerPath, err
	}
	return timerPath, nil
}

func KickstartSystemd() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("systemd service installation is supported on Linux only")
	}
	return runSystemctl("start", SystemdServiceName)
}

func SystemdStatus() (bool, string, error) {
	servicePath, timerPath, err := SystemdPaths()
	if err != nil {
		return false, "", err
	}
	if _, err := os.Stat(servicePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, timerPath, nil
		}
		return false, timerPath, err
	}
	if _, err := os.Stat(timerPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, timerPath, nil
		}
		return false, timerPath, err
	}
	if runtime.GOOS != "linux" {
		return false, timerPath, nil
	}
	err = exec.Command("systemctl", "--user", "is-active", "--quiet", SystemdTimerName).Run()
	return err == nil, timerPath, nil
}

func SystemdPaths() (string, string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", "", err
	}
	dir := filepath.Join(configDir, "systemd", "user")
	return filepath.Join(dir, SystemdServiceName), filepath.Join(dir, SystemdTimerName), nil
}

func SystemdLogsCommand() string {
	return "journalctl --user --unit " + SystemdServiceName
}

func systemdQuote(value string) string {
	return strconv.Quote(strings.ReplaceAll(value, "%", "%%"))
}

func writeSystemdFile(path string, content []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".tlsferry-systemd-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func runSystemctl(arguments ...string) error {
	commandArguments := append([]string{"--user"}, arguments...)
	output, err := exec.Command("systemctl", commandArguments...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl --user %s: %w: %s", strings.Join(arguments, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}
