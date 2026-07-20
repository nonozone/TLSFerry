package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/engine"
	"github.com/nonozone/TLSFerry/internal/preflight"
)

const version = "dev"

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "TLSFerry %s\n", version)
		return 0
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "plan":
		return runPlan(args[1:], stdout, stderr)
	case "preflight":
		return runPreflight(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runPreflight(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("preflight", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "config.json", "path to the TLSFerry configuration")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "preflight failed: %v\n", err)
		return 1
	}
	if err := (preflight.Checker{}).Check(cfg); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	fmt.Fprintf(stdout, "preflight passed: %d certificate(s) are ready\n", len(cfg.Certificates))
	return 0
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "config.json", "path to the TLSFerry configuration")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "invalid configuration: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "configuration is valid: %d certificate(s)\n", len(cfg.Certificates))
	return 0
}

func runPlan(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("plan", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "config.json", "path to the TLSFerry configuration")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "cannot build plan: %v\n", err)
		return 1
	}

	plan := engine.BuildPlan(cfg)
	plan.Render(stdout)
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "TLSFerry automates TLS certificate issuance and multi-cloud delivery.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  tlsferry validate --config config.json")
	fmt.Fprintln(w, "  tlsferry plan     --config config.json")
	fmt.Fprintln(w, "  tlsferry preflight --config config.json")
	fmt.Fprintln(w, "  tlsferry version")
}
