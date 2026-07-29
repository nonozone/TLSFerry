package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var semanticVersionPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

var requiredPublicFiles = []string{
	"CHANGELOG.md",
	"CONTRIBUTING.md",
	"LICENSE",
	"README.md",
	"SECURITY.md",
	"config.example.json",
	"deploy/README.md",
	"docs/edition-boundary.md",
	"docs/release-checklist.md",
	"docs/release-evidence.md",
	"docs/remote-dns-protocol.md",
}

var forbiddenCloudRoots = []string{
	"cloud",
	"apps/cloud-console",
	"internal/account",
	"internal/billing",
	"internal/controlplane",
	"internal/tenant",
	"migrations",
	"wrangler.toml",
}

type check struct {
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	Evidence []string `json:"evidence"`
}

type report struct {
	SchemaVersion int      `json:"schema_version"`
	Status        string   `json:"status"`
	Version       string   `json:"version"`
	Commit        string   `json:"commit"`
	ReviewDate    string   `json:"review_date"`
	Reviewer      string   `json:"reviewer"`
	Checks        []check  `json:"checks"`
	ManualGates   []string `json:"manual_gates"`
}

func main() {
	var version string
	var reviewer string
	var repository string
	flag.StringVar(&version, "version", "", "semantic candidate version, including the leading v")
	flag.StringVar(&reviewer, "reviewer", "", "person or automation identity performing the audit")
	flag.StringVar(&repository, "repository", ".", "path inside the TLSFerry CE repository")
	flag.Parse()

	result, err := audit(repository, version, reviewer)
	if result == nil {
		result = &report{
			SchemaVersion: 1,
			Status:        "fail",
			Version:       version,
			Reviewer:      reviewer,
			ReviewDate:    time.Now().UTC().Format(time.RFC3339),
			Checks: []check{{
				Name:     "audit-input",
				Status:   "fail",
				Evidence: []string{err.Error()},
			}},
		}
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if encodeErr := encoder.Encode(result); encodeErr != nil {
		fmt.Fprintf(os.Stderr, "encode release audit: %v\n", encodeErr)
		os.Exit(1)
	}
	if err != nil || result.Status != "pass" {
		os.Exit(1)
	}
}

func audit(repository, version, reviewer string) (*report, error) {
	if err := validateVersion(version); err != nil {
		return nil, err
	}
	if strings.TrimSpace(reviewer) == "" || strings.ContainsAny(reviewer, "\r\n\x00") {
		return nil, errors.New("reviewer must be non-empty and contain no control characters")
	}

	root, err := git(repository, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("resolve repository root: %w", err)
	}
	root = strings.TrimSpace(root)
	commit, err := git(root, "rev-parse", "HEAD")
	if err != nil {
		return nil, fmt.Errorf("resolve candidate commit: %w", err)
	}
	commit = strings.TrimSpace(commit)
	status, err := git(root, "status", "--porcelain=v1", "--untracked-files=all")
	if err != nil {
		return nil, fmt.Errorf("inspect worktree: %w", err)
	}
	trackedOutput, err := git(root, "ls-files", "-z")
	if err != nil {
		return nil, fmt.Errorf("list tracked files: %w", err)
	}
	tracked := splitNUL(trackedOutput)
	trackedSet := make(map[string]struct{}, len(tracked))
	for _, path := range tracked {
		trackedSet[filepath.ToSlash(path)] = struct{}{}
	}

	checks := []check{
		newCheck("clean-worktree", parseGitStatus(status)),
		newCheck("required-public-files", auditRequiredFiles(root, trackedSet)),
		newCheck("no-sensitive-tracked-files", auditSensitiveTrackedFiles(tracked)),
		newCheck("ce-cloud-boundary", auditCloudBoundary(root, tracked)),
		newCheck("apache-2.0-license", auditLicense(root)),
		newCheck("release-workflow-permissions", auditReleaseWorkflowPermissions(root)),
	}
	result := &report{
		SchemaVersion: 1,
		Status:        "pass",
		Version:       version,
		Commit:        commit,
		ReviewDate:    time.Now().UTC().Format(time.RFC3339),
		Reviewer:      reviewer,
		Checks:        checks,
		ManualGates: []string{
			"GitHub cross-platform CI for this exact commit",
			"public GitHub repository visibility and license metadata",
			"Let's Encrypt staging DNS-01 issuance",
			"non-production cloud provider certificate deployment and rollback",
			"maintainer authorization before pushing a release tag",
		},
	}
	for _, item := range checks {
		if item.Status != "pass" {
			result.Status = "fail"
		}
	}
	return result, nil
}

func newCheck(name string, failures []string) check {
	if len(failures) == 0 {
		return check{Name: name, Status: "pass", Evidence: []string{"verified"}}
	}
	return check{Name: name, Status: "fail", Evidence: failures}
}

func validateVersion(version string) error {
	if !semanticVersionPattern.MatchString(version) {
		return fmt.Errorf("version %q must be semantic version text such as v0.1.0-rc.1", version)
	}
	return nil
}

func git(directory string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return "", errors.New(message)
	}
	return string(output), nil
}

func splitNUL(value string) []string {
	parts := strings.Split(value, "\x00")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func parseGitStatus(status string) []string {
	var failures []string
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 4 {
			continue
		}
		code := line[:2]
		path := decodeGitPath(strings.TrimSpace(line[3:]))
		if code == "??" {
			if sensitiveTrackedPath(path) {
				failures = append(failures, "sensitive untracked file: "+path)
			}
			continue
		}
		if code != "!!" {
			failures = append(failures, "tracked worktree change: "+path)
		}
	}
	sort.Strings(failures)
	return failures
}

func decodeGitPath(path string) string {
	if strings.HasPrefix(path, `"`) {
		if decoded, err := strconv.Unquote(path); err == nil {
			return decoded
		}
	}
	return path
}

func auditRequiredFiles(root string, tracked map[string]struct{}) []string {
	var failures []string
	for _, path := range requiredPublicFiles {
		if _, ok := tracked[path]; !ok {
			failures = append(failures, "required public file is not tracked: "+path)
			continue
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil || info.IsDir() {
			failures = append(failures, "required public file is missing: "+path)
		}
	}
	return failures
}

func auditSensitiveTrackedFiles(tracked []string) []string {
	var failures []string
	for _, path := range tracked {
		if sensitiveTrackedPath(path) {
			failures = append(failures, "sensitive or generated file is tracked: "+filepath.ToSlash(path))
		}
	}
	sort.Strings(failures)
	return failures
}

func sensitiveTrackedPath(path string) bool {
	normalized := strings.TrimPrefix(filepath.ToSlash(path), "./")
	base := strings.ToLower(filepath.Base(normalized))
	if normalized == ".tlsferry" || strings.HasPrefix(normalized, ".tlsferry/") ||
		normalized == "dist" || strings.HasPrefix(normalized, "dist/") ||
		normalized == "bin" || strings.HasPrefix(normalized, "bin/") {
		return true
	}
	if base == "config.json" && normalized != "config.example.json" {
		return true
	}
	switch strings.ToLower(filepath.Ext(base)) {
	case ".pem", ".key", ".crt", ".p12", ".pfx":
		return true
	default:
		return false
	}
}

func auditCloudBoundary(root string, tracked []string) []string {
	failuresByPath := map[string]struct{}{}
	for _, path := range tracked {
		if forbiddenCloudPath(path) {
			failuresByPath[filepath.ToSlash(path)] = struct{}{}
		}
	}
	for _, path := range forbiddenCloudRoots {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err == nil {
			failuresByPath[path] = struct{}{}
		} else if !os.IsNotExist(err) {
			failuresByPath[path+": "+err.Error()] = struct{}{}
		}
	}
	var failures []string
	for path := range failuresByPath {
		failures = append(failures, "Cloud-private path exists in CE: "+path)
	}
	sort.Strings(failures)
	return failures
}

func forbiddenCloudPath(path string) bool {
	normalized := strings.TrimPrefix(filepath.ToSlash(path), "./")
	for _, root := range forbiddenCloudRoots {
		if normalized == root || strings.HasPrefix(normalized, root+"/") {
			return true
		}
	}
	return false
}

func auditLicense(root string) []string {
	content, err := os.ReadFile(filepath.Join(root, "LICENSE"))
	if err != nil {
		return []string{"read LICENSE: " + err.Error()}
	}
	if !bytes.Contains(content, []byte("Apache License")) || !bytes.Contains(content, []byte("Version 2.0, January 2004")) {
		return []string{"LICENSE does not contain the Apache License 2.0 header"}
	}
	return nil
}

func auditReleaseWorkflowPermissions(root string) []string {
	path := filepath.Join(root, ".github", "workflows", "release.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		return []string{"read .github/workflows/release.yml: " + err.Error()}
	}
	workflow := string(content)
	var failures []string

	if !regexp.MustCompile(`(?m)^permissions:\s*\{\}\s*$`).MatchString(workflow) {
		failures = append(failures, "release workflow must deny permissions by default")
	}
	verify, ok := workflowJobBlock(workflow, "verify")
	if !ok {
		failures = append(failures, "release workflow is missing the verify job")
	} else {
		if !strings.Contains(verify, "    permissions:\n      contents: read") {
			failures = append(failures, "release verify job must use contents: read")
		}
		if !strings.Contains(verify, "          persist-credentials: false") {
			failures = append(failures, "release verify checkout must disable credential persistence")
		}
		if strings.Contains(verify, "GITHUB_TOKEN") {
			failures = append(failures, "release verify job must not receive GITHUB_TOKEN")
		}
	}
	publish, ok := workflowJobBlock(workflow, "publish")
	if !ok {
		failures = append(failures, "release workflow is missing the publish job")
	} else {
		if !strings.Contains(publish, "    needs: verify") {
			failures = append(failures, "release publish job must depend on verify")
		}
		if !strings.Contains(publish, "    permissions:\n      contents: write") {
			failures = append(failures, "release publish job must scope contents: write locally")
		}
		if !strings.Contains(publish, "          persist-credentials: false") {
			failures = append(failures, "release publish checkout must disable credential persistence")
		}
		if !strings.Contains(publish, "          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}") {
			failures = append(failures, "release publish action must receive GITHUB_TOKEN explicitly")
		}
	}
	if strings.Count(workflow, "contents: write") != 1 {
		failures = append(failures, "release workflow must contain exactly one contents: write grant")
	}
	if strings.Count(workflow, "          GITHUB_TOKEN:") != 1 {
		failures = append(failures, "release workflow must expose GITHUB_TOKEN exactly once")
	}

	for lineNumber, line := range strings.Split(workflow, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "uses:") {
			continue
		}
		reference := strings.TrimSpace(strings.TrimPrefix(trimmed, "uses:"))
		if comment := strings.Index(reference, " #"); comment >= 0 {
			reference = reference[:comment]
		}
		at := strings.LastIndex(reference, "@")
		if at < 0 || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(reference[at+1:]) {
			failures = append(failures, fmt.Sprintf("release workflow action on line %d is not pinned to a full commit SHA", lineNumber+1))
		}
	}

	sort.Strings(failures)
	return failures
}

func workflowJobBlock(workflow, name string) (string, bool) {
	lines := strings.Split(workflow, "\n")
	start := -1
	for index, line := range lines {
		if line == "  "+name+":" {
			start = index
			break
		}
	}
	if start < 0 {
		return "", false
	}
	end := len(lines)
	for index := start + 1; index < len(lines); index++ {
		line := lines[index]
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    ") && strings.HasSuffix(strings.TrimSpace(line), ":") {
			end = index
			break
		}
	}
	return strings.Join(lines[start:end], "\n"), true
}
