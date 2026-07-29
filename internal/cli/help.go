package cli

import (
	"fmt"
	"io"
	"strings"
)

var commandNames = []string{
	"auth", "completion", "deploy", "discover", "enroll", "help", "issue", "plan", "preflight", "renew", "service", "validate", "version",
}

var commandHelp = map[string]string{
	"auth": `Usage:
	  tlsferry auth login cloudflare|tencent|aliyun|qiniu [--profile PROFILE] [--no-browser]
  tlsferry auth status --profile PROFILE
  tlsferry auth logout --profile PROFILE

Stores cloud credentials in the operating-system credential manager. Secret values are never printed.`,
	"completion": `Usage:
  tlsferry completion zsh|bash|fish
  tlsferry completion install [--shell zsh|bash|fish]

Prints a completion script to stdout, or installs it in the current user's completion directory.`,
	"deploy": `Usage:
  tlsferry deploy --config FILE --certificate NAME --provider PROVIDER --input-dir DIR --execute

Deploys one stored certificate to exactly one configured cloud target. --execute is required.`,
	"discover": `Usage:
  tlsferry discover cloud --provider tencent|aliyun|qiniu [--credential REFERENCE] [--format table|json]

Lists CDN domains with cloud-side status, HTTPS state, and CNAME. This command is read-only.

Options:
  --provider     Required cloud provider
  --credential   env:PROFILE or keychain:PROFILE; defaults to the provider profile
  --format       table (default) or json`,
	"enroll": `Usage:
  tlsferry enroll cloud --provider tencent|aliyun|qiniu --domain DOMAIN --email EMAIL --dns-provider PROVIDER --dns-credential REFERENCE [--credential REFERENCE] [--name NAME] [--config FILE] [--execute]

Verifies one domain against the authorized cloud CDN inventory and previews a certificate configuration. No file is changed unless --execute is present.`,
	"issue": `Usage:
  tlsferry issue --config FILE --certificate NAME [--state-dir DIR] [--output-dir DIR] --accept-tos

Issues one ACME DNS-01 certificate and stores it locally.`,
	"plan": `Usage:
  tlsferry plan --config FILE

Prints the certificate issuance and deployment plan without contacting cloud APIs.`,
	"preflight": `Usage:
  tlsferry preflight --config FILE

Checks configuration, provider support, and credential availability without changing cloud resources.`,
	"renew": `Usage:
  tlsferry renew --config FILE [--certificate NAME] [--retry-attempts N] [--force] --accept-tos --execute

Checks expiry, renews due certificates, and deploys them. Safety flags are mandatory.`,
	"service": `Usage:
  tlsferry service install --config FILE [--hour HOUR] [--minute MINUTE] --accept-tos --execute
  tlsferry service status
  tlsferry service run-now
  tlsferry service logs
  tlsferry service uninstall

Manages the local unattended renewal schedule. Native installation supports macOS launchd and Linux systemd user timers.`,
	"validate": `Usage:
  tlsferry validate --config FILE

Validates configuration syntax and semantics without reading credentials or contacting cloud APIs.`,
	"version": `Usage:
  tlsferry version

Prints the TLSFerry version.`,
}

func hasHelpFlag(args []string) bool {
	for _, argument := range args {
		if argument == "-h" || argument == "--help" {
			return true
		}
	}
	return false
}

func printCommandHelp(command string, stdout, stderr io.Writer) int {
	text, ok := commandHelp[command]
	if !ok {
		fmt.Fprintf(stderr, "help: unknown command %q\n", command)
		if suggestion := suggestCommand(command); suggestion != "" {
			fmt.Fprintf(stderr, "Did you mean %q?\n", suggestion)
		}
		return 2
	}
	fmt.Fprintln(stdout, text)
	return 0
}

func suggestCommand(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	best := ""
	bestDistance := 4
	for _, candidate := range commandNames {
		distance := editDistance(input, candidate)
		if distance < bestDistance {
			best = candidate
			bestDistance = distance
		}
	}
	if bestDistance > 3 {
		return ""
	}
	return best
}

func editDistance(left, right string) int {
	previous := make([]int, len(right)+1)
	for index := range previous {
		previous[index] = index
	}
	for leftIndex, leftRune := range []rune(left) {
		current := make([]int, len(previous))
		current[0] = leftIndex + 1
		for rightIndex, rightRune := range []rune(right) {
			cost := 0
			if leftRune != rightRune {
				cost = 1
			}
			current[rightIndex+1] = min3(current[rightIndex]+1, previous[rightIndex+1]+1, previous[rightIndex]+cost)
		}
		previous = current
	}
	return previous[len(previous)-1]
}

func min3(a, b, c int) int {
	if a < b && a < c {
		return a
	}
	if b < c {
		return b
	}
	return c
}
