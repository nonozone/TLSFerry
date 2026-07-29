package renewal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/nonozone/TLSFerry/internal/acmeissuer"
	"github.com/nonozone/TLSFerry/internal/certstore"
	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/credential"
	"github.com/nonozone/TLSFerry/internal/deployment"
)

type Event struct {
	Certificate string
	Stage       string
	Status      string
	Detail      string
}

type Notifier interface{ Notify(Event) }

type WriterNotifier struct{ Writer io.Writer }

func (n WriterNotifier) Notify(event Event) {
	if n.Writer != nil {
		fmt.Fprintf(n.Writer, "%s: %s %s", event.Certificate, event.Stage, event.Status)
		if event.Detail != "" {
			fmt.Fprintf(n.Writer, " (%s)", event.Detail)
		}
		fmt.Fprintln(n.Writer)
	}
}

type Outcome struct {
	Certificate string
	Status      string
	NotAfter    time.Time
}

type Runner struct {
	StateDir    string
	OutputDir   string
	Credentials credential.EnvResolver
	Attempts    int
	Delay       time.Duration
	Force       bool
	Now         func() time.Time
	Sleep       SleepFunc
	Issue       func(config.Certificate, bool) (certstore.Bundle, error)
	Deploy      func(context.Context, string, config.Deployment, certstore.Bundle) (deployment.Result, error)
}

func (r Runner) Run(ctx context.Context, cfg config.Config, certificateFilter string, acceptTerms bool, notifier Notifier) ([]Outcome, error) {
	release, err := AcquireLock(r.StateDir)
	if err != nil {
		return nil, err
	}
	defer release()
	renewBefore, err := time.ParseDuration(cfg.RenewBefore)
	if err != nil {
		return nil, err
	}
	attempts := r.Attempts
	if attempts == 0 {
		attempts = 3
	}
	delay := r.Delay
	if delay == 0 {
		delay = 2 * time.Second
	}
	now := r.Now
	if now == nil {
		now = time.Now
	}
	store := certstore.Store{Root: r.OutputDir}
	issue := r.Issue
	if issue == nil {
		client := acmeissuer.Client{StateDir: r.StateDir, Credentials: r.Credentials}
		issue = client.Obtain
	}
	deploy := r.Deploy
	if deploy == nil {
		manager := deployment.NewManager(r.Credentials)
		deploy = manager.Deploy
	}

	var outcomes []Outcome
	matched := false
	for _, certificateConfig := range cfg.Certificates {
		if certificateFilter != "" && certificateConfig.Name != certificateFilter {
			continue
		}
		matched = true
		bundle, loadErr := store.Load(certificateConfig.Name)
		due := r.Force
		var notAfter time.Time
		if !due {
			switch {
			case loadErr == nil:
				due, notAfter, err = NeedsRenewal(bundle, renewBefore, now())
				if err != nil {
					return outcomes, fmt.Errorf("check certificate %q: %w", certificateConfig.Name, err)
				}
			case errors.Is(loadErr, os.ErrNotExist):
				due = true
			default:
				return outcomes, fmt.Errorf("load certificate %q: %w", certificateConfig.Name, loadErr)
			}
		}
		if !due {
			outcomes = append(outcomes, Outcome{certificateConfig.Name, "skipped", notAfter})
			notify(notifier, Event{certificateConfig.Name, "renewal", "skipped", "certificate is outside the renewal window"})
			continue
		}

		notify(notifier, Event{certificateConfig.Name, "issuance", "started", ""})
		var issued certstore.Bundle
		err = Retry(ctx, attempts, delay, r.Sleep, func() error {
			var issueErr error
			issued, issueErr = issue(certificateConfig, acceptTerms)
			return issueErr
		})
		if err != nil {
			notify(notifier, Event{certificateConfig.Name, "issuance", "failed", err.Error()})
			return outcomes, err
		}
		if _, err := store.Save(certificateConfig.Name, issued); err != nil {
			return outcomes, err
		}
		notify(notifier, Event{certificateConfig.Name, "issuance", "completed", ""})

		for _, deploymentConfig := range certificateConfig.Deployments {
			var result deployment.Result
			err = Retry(ctx, attempts, delay, r.Sleep, func() error {
				var deployErr error
				result, deployErr = deploy(ctx, certificateConfig.Name, deploymentConfig, issued)
				return deployErr
			})
			if err != nil {
				notify(notifier, Event{certificateConfig.Name, deploymentConfig.Provider, "failed", err.Error()})
				return outcomes, err
			}
			notify(notifier, Event{certificateConfig.Name, deploymentConfig.Provider, result.Status, result.Reference})
		}
		outcomes = append(outcomes, Outcome{certificateConfig.Name, "renewed", time.Time{}})
	}
	if certificateFilter != "" && !matched {
		return outcomes, fmt.Errorf("certificate %q was not found", certificateFilter)
	}
	return outcomes, nil
}

func notify(notifier Notifier, event Event) {
	if notifier != nil {
		notifier.Notify(event)
	}
}
