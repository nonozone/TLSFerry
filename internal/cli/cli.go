package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/nonozone/TLSFerry/internal/acmeissuer"
	"github.com/nonozone/TLSFerry/internal/certstore"
	"github.com/nonozone/TLSFerry/internal/config"
	"github.com/nonozone/TLSFerry/internal/credential"
	"github.com/nonozone/TLSFerry/internal/deployment"
	"github.com/nonozone/TLSFerry/internal/discovery"
	"github.com/nonozone/TLSFerry/internal/engine"
	"github.com/nonozone/TLSFerry/internal/preflight"
	"github.com/nonozone/TLSFerry/internal/renewal"
	"github.com/nonozone/TLSFerry/internal/service"
	"golang.org/x/term"
)

// version is replaced at release time with -ldflags. Local builds keep the
// explicit dev value so operators can distinguish them from published builds.
var version = "dev"

func Run(args []string, stdout, stderr io.Writer) int {
	return RunWithInput(args, os.Stdin, stdout, stderr)
}

func RunWithInput(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	if args[0] == "help" {
		if len(args) == 1 {
			printUsage(stdout)
			return 0
		}
		return printCommandHelp(args[1], stdout, stderr)
	}
	if hasHelpFlag(args[1:]) {
		return printCommandHelp(args[0], stdout, stderr)
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
	case "auth":
		return runAuth(args[1:], stdin, stdout, stderr)
	case "service":
		return runService(args[1:], stdout, stderr)
	case "discover":
		return runDiscover(args[1:], stdout, stderr)
	case "completion":
		return runCompletion(args[1:], stdout, stderr)
	case "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		if suggestion := suggestCommand(args[0]); suggestion != "" {
			fmt.Fprintf(stderr, "Did you mean %q?\n", suggestion)
		}
		fmt.Fprintln(stderr)
		printUsage(stderr)
		return 2
	}
}

var newCloudScanner = discovery.NewScanner

func runDiscover(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "cloud" {
		fmt.Fprintln(stderr, "discover: expected cloud")
		return 2
	}
	flags := flag.NewFlagSet("discover cloud", flag.ContinueOnError)
	flags.SetOutput(stderr)
	provider := flags.String("provider", "", "cloud provider: tencent, aliyun, or qiniu")
	reference := flags.String("credential", "", "credential reference; defaults to the provider keychain profile")
	format := flags.String("format", "table", "output format: table or json")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	*provider = strings.ToLower(strings.TrimSpace(*provider))
	if *provider == "" {
		fmt.Fprintln(stderr, "discover cloud: --provider is required")
		return 2
	}
	if *reference == "" {
		profiles := map[string]string{
			"tencent": "keychain:TENCENTCLOUD",
			"aliyun":  "keychain:ALIYUN",
			"qiniu":   "keychain:QINIU",
		}
		*reference = profiles[*provider]
	}
	if *reference == "" {
		fmt.Fprintf(stderr, "discover cloud: unsupported provider %q\n", *provider)
		return 2
	}
	if *format != "table" && *format != "json" {
		fmt.Fprintln(stderr, "discover cloud: --format must be table or json")
		return 2
	}
	scanner, err := newCloudScanner(*provider, credential.Resolver{}, *reference)
	if err != nil {
		fmt.Fprintf(stderr, "discover cloud: %v\n", err)
		return 2
	}
	domains, err := (discovery.Manager{Scanners: map[string]discovery.Scanner{*provider: scanner}}).Scan(context.Background(), *provider)
	if err != nil {
		fmt.Fprintf(stderr, "discover cloud failed: %v\n", err)
		return 1
	}
	if *format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(domains); err != nil {
			fmt.Fprintf(stderr, "discover cloud: write output: %v\n", err)
			return 1
		}
		return 0
	}
	table := tabwriter.NewWriter(stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "PROVIDER\tDOMAIN\tSTATUS\tHTTPS\tCNAME")
	for _, domain := range domains {
		httpsStatus := "off"
		if domain.HTTPS {
			httpsStatus = "on"
		}
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", domain.Provider, domain.Name, domain.Status, httpsStatus, domain.CNAME)
	}
	if err := table.Flush(); err != nil {
		fmt.Fprintf(stderr, "discover cloud: write output: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "\nDiscovered %d CDN domain(s). Read-only scan; no cloud resources were changed.\n", len(domains))
	return 0
}

type authProvider struct {
	Profile string
	URL     string
	Fields  []string
}

var authProviders = map[string]authProvider{
	"cloudflare": {Profile: "CLOUDFLARE", URL: "https://dash.cloudflare.com/profile/api-tokens", Fields: []string{"API_TOKEN"}},
	"tencent":    {Profile: "TENCENTCLOUD", URL: "https://console.cloud.tencent.com/cam/capi", Fields: []string{"SECRET_ID", "SECRET_KEY"}},
	"aliyun":     {Profile: "ALIYUN", URL: "https://ram.console.aliyun.com/manage/ak", Fields: []string{"ACCESS_KEY_ID", "ACCESS_KEY_SECRET"}},
	"qiniu":      {Profile: "QINIU", URL: "https://portal.qiniu.com/user/key", Fields: []string{"ACCESS_KEY", "SECRET_KEY"}},
}

var newCredentialStore = func() credential.Store { return credential.KeyringStore{} }
var openCredentialURL = openURL

func runAuth(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "auth: expected login, status, or logout")
		return 2
	}
	switch args[0] {
	case "login":
		return runAuthLogin(args[1:], stdin, stdout, stderr)
	case "status":
		return runAuthStatus(args[1:], stdout, stderr)
	case "logout":
		return runAuthLogout(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "auth: unknown action %q\n", args[0])
		return 2
	}
}

func runAuthLogin(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "auth login: provider is required (cloudflare, tencent, aliyun, or qiniu)")
		return 2
	}
	provider, ok := authProviders[strings.ToLower(args[0])]
	if !ok {
		fmt.Fprintf(stderr, "auth login: unsupported provider %q\n", args[0])
		return 2
	}
	flags := flag.NewFlagSet("auth login", flag.ContinueOnError)
	flags.SetOutput(stderr)
	profile := flags.String("profile", provider.Profile, "keychain profile name")
	noBrowser := flags.Bool("no-browser", false, "print the cloud console URL without opening it")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	if _, err := credential.ParseKeychainReference("keychain:" + *profile); err != nil {
		fmt.Fprintf(stderr, "auth login: %v\n", err)
		return 2
	}

	fmt.Fprintf(stdout, "Cloud credential page: %s\n", provider.URL)
	if !*noBrowser {
		if err := openCredentialURL(provider.URL); err != nil {
			fmt.Fprintf(stderr, "auth login: browser could not be opened: %v\n", err)
			fmt.Fprintln(stderr, "Open the URL shown above and continue here.")
		}
	}
	reader := bufio.NewReader(stdin)
	values := make(map[string]string, len(provider.Fields))
	for _, field := range provider.Fields {
		value, err := readCredentialValue(stdin, reader, stdout, field)
		if err != nil {
			fmt.Fprintf(stderr, "auth login: read %s: %v\n", field, err)
			return 1
		}
		if value == "" {
			fmt.Fprintf(stderr, "auth login: %s cannot be empty\n", field)
			return 2
		}
		values[field] = value
	}
	if err := newCredentialStore().Set(*profile, values); err != nil {
		fmt.Fprintf(stderr, "auth login: save credential: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Credential saved as keychain:%s. Secret values were not written to config.json.\n", *profile)
	return 0
}

func runAuthStatus(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("auth status", flag.ContinueOnError)
	flags.SetOutput(stderr)
	profile := flags.String("profile", "", "keychain profile name")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *profile == "" {
		fmt.Fprintln(stderr, "auth status: --profile is required")
		return 2
	}
	values, err := newCredentialStore().Get(*profile)
	if err != nil {
		fmt.Fprintf(stderr, "auth status: %v\n", err)
		return 1
	}
	fields := make([]string, 0, len(values))
	for field := range values {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	fmt.Fprintf(stdout, "keychain:%s is configured (%s)\n", *profile, strings.Join(fields, ", "))
	return 0
}

func runAuthLogout(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("auth logout", flag.ContinueOnError)
	flags.SetOutput(stderr)
	profile := flags.String("profile", "", "keychain profile name")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *profile == "" {
		fmt.Fprintln(stderr, "auth logout: --profile is required")
		return 2
	}
	if err := newCredentialStore().Delete(*profile); err != nil {
		fmt.Fprintf(stderr, "auth logout: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "keychain:%s was removed\n", *profile)
	return 0
}

func readCredentialValue(stdin io.Reader, reader *bufio.Reader, stdout io.Writer, field string) (string, error) {
	fmt.Fprintf(stdout, "%s: ", field)
	if file, ok := stdin.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		value, err := term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(stdout)
		return strings.TrimSpace(string(value)), err
	}
	value, err := reader.ReadString('\n')
	return strings.TrimSpace(value), err
}

func openURL(url string) error {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.Command("open", url)
	case "windows":
		command = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		command = exec.Command("xdg-open", url)
	}
	return command.Start()
}

func runService(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "service: expected install, status, run-now, logs, or uninstall")
		return 2
	}
	switch args[0] {
	case "install":
		return runServiceInstall(args[1:], stdout, stderr)
	case "status":
		var running bool
		var path string
		var err error
		switch runtime.GOOS {
		case "darwin":
			running, path, err = service.LaunchdStatus()
		case "linux":
			running, path, err = service.SystemdStatus()
		default:
			err = fmt.Errorf("automatic service installation is not supported on %s", runtime.GOOS)
		}
		if err != nil {
			fmt.Fprintf(stderr, "service status: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "service installed: %t\nservice running: %t\nschedule: %s\n", fileExists(path), running, path)
		return 0
	case "run-now":
		var err error
		switch runtime.GOOS {
		case "darwin":
			err = service.KickstartLaunchd()
		case "linux":
			err = service.KickstartSystemd()
		default:
			err = fmt.Errorf("automatic service installation is not supported on %s", runtime.GOOS)
		}
		if err != nil {
			fmt.Fprintf(stderr, "service run-now: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "renewal service started")
		return 0
	case "logs":
		if runtime.GOOS == "linux" {
			fmt.Fprintf(stdout, "renewal logs: %s\n", service.SystemdLogsCommand())
			return 0
		}
		if runtime.GOOS != "darwin" {
			fmt.Fprintf(stderr, "service logs: automatic service installation is not supported on %s\n", runtime.GOOS)
			return 1
		}
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(stderr, "service logs: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "renewal log: %s\nerror log: %s\n", filepath.Join(home, ".tlsferry", "logs", "renew.log"), filepath.Join(home, ".tlsferry", "logs", "renew.error.log"))
		return 0
	case "uninstall":
		var path string
		var err error
		switch runtime.GOOS {
		case "darwin":
			path, err = service.UninstallLaunchd()
		case "linux":
			path, err = service.UninstallSystemd()
		default:
			err = fmt.Errorf("automatic service installation is not supported on %s", runtime.GOOS)
		}
		if err != nil {
			fmt.Fprintf(stderr, "service uninstall: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "renewal service removed: %s\n", path)
		return 0
	default:
		fmt.Fprintf(stderr, "service: unknown action %q\n", args[0])
		return 2
	}
}

func runServiceInstall(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("service install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "config.json", "path to the TLSFerry configuration")
	stateDir := flags.String("state-dir", ".tlsferry", "directory for ACME and renewal state")
	outputDir := flags.String("output-dir", ".tlsferry/certificates", "directory containing issued certificates")
	hour := flags.Int("hour", 3, "daily renewal check hour")
	minute := flags.Int("minute", 17, "daily renewal check minute")
	acceptTerms := flags.Bool("accept-tos", false, "accept the ACME provider terms of service for scheduled renewals")
	execute := flags.Bool("execute", false, "allow scheduled external ACME, DNS, and cloud operations")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if !*acceptTerms || !*execute {
		fmt.Fprintln(stderr, "service install: --accept-tos and --execute are required")
		return 2
	}
	if _, err := config.Load(*configPath); err != nil {
		fmt.Fprintf(stderr, "service install: %v\n", err)
		return 1
	}
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "service install: %v\n", err)
		return 1
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(stderr, "service install: %v\n", err)
		return 1
	}
	absolute := func(path string) (string, bool) {
		result, pathErr := filepath.Abs(path)
		if pathErr != nil {
			fmt.Fprintf(stderr, "service install: %v\n", pathErr)
			return "", false
		}
		return result, true
	}
	absoluteConfig, ok := absolute(*configPath)
	if !ok {
		return 1
	}
	absoluteState, ok := absolute(*stateDir)
	if !ok {
		return 1
	}
	absoluteOutput, ok := absolute(*outputDir)
	if !ok {
		return 1
	}
	var path string
	switch runtime.GOOS {
	case "darwin":
		path, err = service.InstallLaunchd(service.LaunchdConfig{
			Executable: executable,
			ConfigPath: absoluteConfig,
			StateDir:   absoluteState,
			OutputDir:  absoluteOutput,
			LogDir:     filepath.Join(home, ".tlsferry", "logs"),
			Hour:       *hour,
			Minute:     *minute,
		})
	case "linux":
		path, err = service.InstallSystemd(service.SystemdConfig{
			Executable: executable,
			ConfigPath: absoluteConfig,
			StateDir:   absoluteState,
			OutputDir:  absoluteOutput,
			Hour:       *hour,
			Minute:     *minute,
		})
	default:
		err = fmt.Errorf("automatic service installation is not supported on %s", runtime.GOOS)
	}
	if err != nil {
		fmt.Fprintf(stderr, "service install: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "renewal service installed: %s\ndaily check: %02d:%02d\n", path, *hour, *minute)
	return 0
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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
	fmt.Fprintln(w, "  tlsferry auth login cloudflare|tencent|aliyun|qiniu")
	fmt.Fprintln(w, "  tlsferry auth status --profile PROFILE")
	fmt.Fprintln(w, "  tlsferry service install --config config.json --accept-tos --execute")
	fmt.Fprintln(w, "  tlsferry service status|run-now|logs|uninstall")
	fmt.Fprintln(w, "  tlsferry discover cloud --provider tencent|aliyun|qiniu")
	fmt.Fprintln(w, "  tlsferry completion zsh|bash|fish")
	fmt.Fprintln(w, "  tlsferry help COMMAND")
	fmt.Fprintln(w, "  tlsferry version")
}
