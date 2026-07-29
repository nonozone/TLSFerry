package cli

import (
	"strings"
	"testing"
)

func TestIssueRequiresCertificateName(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"issue", "--accept-tos"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "--certificate is required") {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}
}

func TestIssueRequiresTermsAcceptance(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"issue", "--certificate", "assets"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "--accept-tos is required") {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}
}

func TestDeployRequiresProvider(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"deploy", "--certificate", "assets", "--execute"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "--provider is required") {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}
}

func TestDeployRequiresExplicitExecution(t *testing.T) {
	var stdout, stderr strings.Builder
	code := Run([]string{"deploy", "--certificate", "assets", "--provider", "tencent-cdn"}, &stdout, &stderr)
	if code != 2 || !strings.Contains(stderr.String(), "--execute is required") {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}
}

func TestRenewRequiresSafetyFlags(t *testing.T) {
	var stdout, stderr strings.Builder
	if code := Run([]string{"renew"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "--accept-tos") {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}
	stderr.Reset()
	if code := Run([]string{"renew", "--accept-tos"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "--execute") {
		t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
	}
}
