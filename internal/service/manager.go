package service

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
)

func InstallLaunchd(config LaunchdConfig) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("automatic service installation is currently supported on macOS only")
	}
	plist, err := RenderLaunchd(config)
	if err != nil {
		return "", err
	}
	path, err := LaunchdPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(config.LogDir, 0o700); err != nil {
		return "", err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".tlsferry-launchd-*")
	if err != nil {
		return "", err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return "", err
	}
	if _, err := temporary.Write(plist); err != nil {
		temporary.Close()
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return "", err
	}

	domain := launchdDomain()
	_ = exec.Command("launchctl", "bootout", domain+"/"+LaunchdLabel).Run()
	if output, err := exec.Command("launchctl", "bootstrap", domain, path).CombinedOutput(); err != nil {
		return path, fmt.Errorf("load launchd service: %w: %s", err, string(output))
	}
	return path, nil
}

func UninstallLaunchd() (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("automatic service installation is currently supported on macOS only")
	}
	path, err := LaunchdPath()
	if err != nil {
		return "", err
	}
	_ = exec.Command("launchctl", "bootout", launchdDomain()+"/"+LaunchdLabel).Run()
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return path, err
	}
	return path, nil
}

func KickstartLaunchd() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("automatic service installation is currently supported on macOS only")
	}
	output, err := exec.Command("launchctl", "kickstart", "-k", launchdDomain()+"/"+LaunchdLabel).CombinedOutput()
	if err != nil {
		return fmt.Errorf("start launchd service: %w: %s", err, string(output))
	}
	return nil
}

func LaunchdStatus() (bool, string, error) {
	path, err := LaunchdPath()
	if err != nil {
		return false, "", err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return false, path, nil
	} else if err != nil {
		return false, path, err
	}
	if runtime.GOOS != "darwin" {
		return false, path, nil
	}
	err = exec.Command("launchctl", "print", launchdDomain()+"/"+LaunchdLabel).Run()
	return err == nil, path, nil
}

func LaunchdPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", LaunchdLabel+".plist"), nil
}

func launchdDomain() string {
	return "gui/" + strconv.Itoa(os.Getuid())
}
