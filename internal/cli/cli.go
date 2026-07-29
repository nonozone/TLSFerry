package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/nonozone/TLSFerry/internal/acmeissuer"
	"github.com/nonozone/TLSFerry/internal/certstore"
	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/credential"
	"github.com/nonozone/TLSFerry/internal/deployment"
	"github.com/nonozone/TLSFerry/internal/engine"
	"github.com/nonozone/TLSFerry/internal/preflight"
	"github.com/nonozone/TLSFerry/internal/renewal"
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
	case "issue":
		return runIssue(args[1:], stdout, stderr)
	case "deploy":
		return runDeploy(args[1:], stdout, stderr)
	case "renew":
		return runRenew(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runRenew(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("renew", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "config.json", "path to the TLSFerry configuration")
	certificateName := flags.String("certificate", "", "optional certificate name; defaults to all certificates")
	stateDir := flags.String("state-dir", ".tlsferry", "directory for ACME and renewal state")
	outputDir := flags.String("output-dir", ".tlsferry/certificates", "directory containing issued certificates")
	attempts := flags.Int("retry-attempts", 3, "maximum attempts for each network operation")
	force := flags.Bool("force", false, "renew even when the certificate is outside its renewal window")
	acceptTerms := flags.Bool("accept-tos", false, "accept the ACME provider terms of service")
	execute := flags.Bool("execute", false, "perform external ACME, DNS, and cloud operations")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if !*acceptTerms {
		fmt.Fprintln(stderr, "renew: --accept-tos is required")
		return 2
	}
	if !*execute {
		fmt.Fprintln(stderr, "renew: --execute is required")
		return 2
	}
	if *attempts < 1 {
		fmt.Fprintln(stderr, "renew: --retry-attempts must be positive")
		return 2
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "renew failed: %v\n", err)
		return 1
	}
	runner := renewal.Runner{StateDir: *stateDir, OutputDir: *outputDir, Attempts: *attempts, Force: *force}
	if _, err := runner.Run(context.Background(), cfg, *certificateName, true, renewal.WriterNotifier{Writer: stdout}); err != nil {
		fmt.Fprintf(stderr, "renew failed: %v\n", err)
		return 1
	}
	return 0
}

func runDeploy(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("deploy", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "config.json", "path to the TLSFerry configuration")
	certificateName := flags.String("certificate", "", "name of the certificate to deploy")
	providerName := flags.String("provider", "", "configured deployment provider to execute")
	inputDir := flags.String("input-dir", ".tlsferry/certificates", "directory containing issued certificate files")
	execute := flags.Bool("execute", false, "perform the external cloud deployment")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *certificateName == "" {
		fmt.Fprintln(stderr, "deploy: --certificate is required")
		return 2
	}
	if *providerName == "" {
		fmt.Fprintln(stderr, "deploy: --provider is required")
		return 2
	}
	if !*execute {
		fmt.Fprintln(stderr, "deploy: --execute is required")
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "deploy failed: %v\n", err)
		return 1
	}
	certificateConfig, ok := findCertificate(cfg, *certificateName)
	if !ok {
		fmt.Fprintf(stderr, "deploy failed: certificate %q was not found\n", *certificateName)
		return 1
	}
	deploymentConfig, ok := findDeployment(certificateConfig, *providerName)
	if !ok {
		fmt.Fprintf(stderr, "deploy failed: provider %q is not configured for certificate %q\n", *providerName, *certificateName)
		return 1
	}
	bundle, err := (certstore.Store{Root: *inputDir}).Load(certificateConfig.Name)
	if err != nil {
		fmt.Fprintf(stderr, "deploy failed: %v\n", err)
		return 1
	}
	result, err := deployment.NewManager(credential.EnvResolver{}).Deploy(context.Background(), certificateConfig.Name, deploymentConfig, bundle)
	if err != nil {
		fmt.Fprintf(stderr, "deploy failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "deployment %s: %s -> %s\n", result.Status, result.Provider, result.Target)
	fmt.Fprintf(stdout, "  reference: %s\n", result.Reference)
	return 0
}

func findDeployment(certificate config.Certificate, provider string) (config.Deployment, bool) {
	for _, candidate := range certificate.Deployments {
		if candidate.Provider == provider {
			return candidate, true
		}
	}
	return config.Deployment{}, false
}

func runIssue(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("issue", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "config.json", "path to the TLSFerry configuration")
	certificateName := flags.String("certificate", "", "name of the certificate to issue")
	stateDir := flags.String("state-dir", ".tlsferry", "directory for persistent ACME account state")
	outputDir := flags.String("output-dir", ".tlsferry/certificates", "directory for issued certificate files")
	acceptTerms := flags.Bool("accept-tos", false, "accept the ACME provider terms of service")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *certificateName == "" {
		fmt.Fprintln(stderr, "issue: --certificate is required")
		return 2
	}
	if !*acceptTerms {
		fmt.Fprintln(stderr, "issue: --accept-tos is required")
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "issue failed: %v\n", err)
		return 1
	}
	certificateConfig, ok := findCertificate(cfg, *certificateName)
	if !ok {
		fmt.Fprintf(stderr, "issue failed: certificate %q was not found\n", *certificateName)
		return 1
	}

	bundle, err := (acmeissuer.Client{StateDir: *stateDir}).Obtain(certificateConfig, true)
	if err != nil {
		fmt.Fprintf(stderr, "issue failed: %v\n", err)
		return 1
	}
	paths, err := (certstore.Store{Root: *outputDir}).Save(certificateConfig.Name, bundle)
	if err != nil {
		fmt.Fprintf(stderr, "issue failed while saving certificate: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "certificate %q issued successfully\n", certificateConfig.Name)
	fmt.Fprintf(stdout, "  certificate: %s\n", paths.Certificate)
	fmt.Fprintf(stdout, "  full chain:  %s\n", paths.FullChain)
	fmt.Fprintf(stdout, "  private key: %s\n", paths.PrivateKey)
	return 0
}

func findCertificate(cfg config.Config, name string) (config.Certificate, bool) {
	for _, certificate := range cfg.Certificates {
		if certificate.Name == name {
			return certificate, true
		}
	}
	return config.Certificate{}, false
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
	fmt.Fprintln(w, "  tlsferry issue --config config.json --certificate NAME --accept-tos")
	fmt.Fprintln(w, "  tlsferry deploy --config config.json --certificate NAME --provider PROVIDER --execute")
	fmt.Fprintln(w, "  tlsferry renew --config config.json --accept-tos --execute")
	fmt.Fprintln(w, "  tlsferry version")
}
