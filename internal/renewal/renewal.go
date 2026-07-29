package renewal

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nonozone/TLSFerry/internal/certstore"
)

func NeedsRenewal(bundle certstore.Bundle, renewBefore time.Duration, now time.Time) (bool, time.Time, error) {
	block, _ := pem.Decode(bundle.Certificate)
	if block == nil || block.Type != "CERTIFICATE" {
		return false, time.Time{}, errors.New("stored certificate is not valid PEM")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, time.Time{}, fmt.Errorf("parse stored certificate: %w", err)
	}
	return !certificate.NotAfter.After(now.Add(renewBefore)), certificate.NotAfter, nil
}

type SleepFunc func(context.Context, time.Duration) error

func Retry(ctx context.Context, attempts int, delay time.Duration, sleep SleepFunc, operation func() error) error {
	if attempts < 1 {
		return errors.New("retry attempts must be positive")
	}
	if sleep == nil {
		sleep = sleepContext
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := operation(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt < attempts {
			if err := sleep(ctx, delay*time.Duration(attempt)); err != nil {
				return err
			}
		}
	}
	return fmt.Errorf("operation failed after %d attempt(s): %w", attempts, lastErr)
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func AcquireLock(stateDir string) (func() error, error) {
	if stateDir == "" {
		return nil, errors.New("renewal state directory is required")
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, fmt.Errorf("create renewal state directory: %w", err)
	}
	lockPath := filepath.Join(stateDir, "renew.lock")
	if err := os.Mkdir(lockPath, 0o700); err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, errors.New("another renewal process is already running")
		}
		return nil, fmt.Errorf("acquire renewal lock: %w", err)
	}
	return func() error {
		if err := os.Remove(lockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("release renewal lock: %w", err)
		}
		return nil
	}, nil
}
