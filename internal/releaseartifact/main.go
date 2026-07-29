// Command releaseartifact verifies TLSFerry CE archives produced by GoReleaser.
package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const maxArchiveEntrySize = 128 << 20

var bundledPublicFiles = []string{"LICENSE", "README.md", "config.example.json", "config.release-smoke.example.json"}

type releaseMetadata struct {
	ProjectName string `json:"project_name"`
	Version     string `json:"version"`
	Commit      string `json:"commit"`
}

type archiveEntry struct {
	Data []byte
	Mode fs.FileMode
}

type archiveEvidence struct {
	Name     string   `json:"name"`
	SHA256   string   `json:"sha256"`
	Contents []string `json:"contents"`
}

type report struct {
	SchemaVersion int               `json:"schema_version"`
	Status        string            `json:"status"`
	Version       string            `json:"version,omitempty"`
	Commit        string            `json:"commit,omitempty"`
	Archives      []archiveEvidence `json:"archives,omitempty"`
	NativeArchive string            `json:"native_archive,omitempty"`
	NativeOutput  string            `json:"native_output,omitempty"`
	Error         string            `json:"error,omitempty"`
}

func main() {
	repository := flag.String("repository", ".", "TLSFerry CE repository root")
	dist := flag.String("dist", "dist", "GoReleaser distribution directory")
	flag.Parse()

	result, err := audit(*repository, *dist)
	if err != nil {
		result = &report{SchemaVersion: 1, Status: "fail", Error: err.Error()}
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if encodeErr := encoder.Encode(result); encodeErr != nil {
		fmt.Fprintf(os.Stderr, "encode artifact report: %v\n", encodeErr)
		os.Exit(1)
	}
	if err != nil {
		os.Exit(1)
	}
}

func audit(repository, dist string) (*report, error) {
	root, err := filepath.Abs(repository)
	if err != nil {
		return nil, fmt.Errorf("resolve repository: %w", err)
	}
	distPath := dist
	if !filepath.IsAbs(distPath) {
		distPath = filepath.Join(root, distPath)
	}

	metadata, err := loadMetadata(filepath.Join(distPath, "metadata.json"))
	if err != nil {
		return nil, err
	}
	head, err := gitHead(root)
	if err != nil {
		return nil, err
	}
	if err := requireCleanTrackedWorktree(root); err != nil {
		return nil, err
	}
	if metadata.Commit != head {
		return nil, fmt.Errorf("artifact commit %s does not match repository HEAD %s", metadata.Commit, head)
	}

	checksumsFile, err := os.Open(filepath.Join(distPath, "checksums.txt"))
	if err != nil {
		return nil, fmt.Errorf("open checksums.txt: %w", err)
	}
	checksums, checksumErr := parseChecksums(checksumsFile)
	closeErr := checksumsFile.Close()
	if checksumErr != nil {
		return nil, checksumErr
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close checksums.txt: %w", closeErr)
	}

	expectedNames := expectedArchiveNames(metadata.Version)
	if err := validateChecksumCoverage(checksums, expectedNames); err != nil {
		return nil, err
	}
	source, err := loadBundledSource(root)
	if err != nil {
		return nil, err
	}

	nativeSuffix, nativeBinaryName, err := nativeArchiveIdentity()
	if err != nil {
		return nil, err
	}
	result := &report{SchemaVersion: 1, Status: "pass", Version: metadata.Version, Commit: metadata.Commit}
	var nativeBinary []byte
	for _, name := range expectedNames {
		archivePath := filepath.Join(distPath, name)
		actualHash, err := hashRegularFile(archivePath)
		if err != nil {
			return nil, err
		}
		if actualHash != checksums[name] {
			return nil, fmt.Errorf("checksum mismatch for %s", name)
		}

		binaryName := "tlsferry"
		requireExecutable := true
		if strings.HasSuffix(name, ".zip") {
			binaryName = "tlsferry.exe"
			requireExecutable = false
		}
		entries, err := readArchive(archivePath)
		if err != nil {
			return nil, fmt.Errorf("inspect %s: %w", name, err)
		}
		binary, err := validateArchiveEntries(entries, binaryName, source, requireExecutable)
		if err != nil {
			return nil, fmt.Errorf("inspect %s: %w", name, err)
		}
		contents := make([]string, 0, len(entries))
		for entryName := range entries {
			contents = append(contents, entryName)
		}
		sort.Strings(contents)
		result.Archives = append(result.Archives, archiveEvidence{Name: name, SHA256: actualHash, Contents: contents})
		if strings.HasSuffix(name, nativeSuffix) {
			result.NativeArchive = name
			nativeBinary = binary
		}
	}
	if len(nativeBinary) == 0 {
		return nil, fmt.Errorf("native archive %q was not found", nativeSuffix)
	}
	output, err := executeVersion(nativeBinary, nativeBinaryName)
	if err != nil {
		return nil, err
	}
	expectedOutput := "TLSFerry " + metadata.Version
	if output != expectedOutput {
		return nil, fmt.Errorf("native version output %q does not match %q", output, expectedOutput)
	}
	result.NativeOutput = output
	return result, nil
}

func loadMetadata(path string) (releaseMetadata, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return releaseMetadata{}, fmt.Errorf("read metadata.json: %w", err)
	}
	var metadata releaseMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		return releaseMetadata{}, fmt.Errorf("decode metadata.json: %w", err)
	}
	if metadata.ProjectName != "tlsferry" {
		return releaseMetadata{}, fmt.Errorf("unexpected project name %q", metadata.ProjectName)
	}
	if strings.TrimSpace(metadata.Version) == "" || strings.ContainsAny(metadata.Version, "/\\\r\n\x00") {
		return releaseMetadata{}, fmt.Errorf("unsafe or empty release version %q", metadata.Version)
	}
	if len(metadata.Commit) != 40 {
		return releaseMetadata{}, fmt.Errorf("release commit must be a full SHA-1, got %q", metadata.Commit)
	}
	if _, err := hex.DecodeString(metadata.Commit); err != nil {
		return releaseMetadata{}, fmt.Errorf("invalid release commit %q", metadata.Commit)
	}
	return metadata, nil
}

func gitHead(root string) (string, error) {
	output, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolve repository HEAD: %s", strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func requireCleanTrackedWorktree(root string) error {
	output, err := exec.Command("git", "-C", root, "status", "--porcelain=v1", "--untracked-files=no").CombinedOutput()
	if err != nil {
		return fmt.Errorf("inspect repository worktree: %s", strings.TrimSpace(string(output)))
	}
	if strings.TrimSpace(string(output)) != "" {
		return errors.New("artifact verification requires a clean tracked worktree")
	}
	return nil
}

func parseChecksums(reader io.Reader) (map[string]string, error) {
	checksums := map[string]string{}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 {
			return nil, fmt.Errorf("checksums.txt line %d must contain one hash and one file name", lineNumber)
		}
		hash := strings.ToLower(fields[0])
		decoded, err := hex.DecodeString(hash)
		if err != nil || len(decoded) != sha256.Size {
			return nil, fmt.Errorf("checksums.txt line %d has an invalid SHA-256", lineNumber)
		}
		name := fields[1]
		if !safeArchiveEntryName(name) {
			return nil, fmt.Errorf("checksums.txt line %d has unsafe file name %q", lineNumber, name)
		}
		if _, exists := checksums[name]; exists {
			return nil, fmt.Errorf("checksums.txt contains duplicate file %q", name)
		}
		checksums[name] = hash
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read checksums.txt: %w", err)
	}
	if len(checksums) == 0 {
		return nil, errors.New("checksums.txt is empty")
	}
	return checksums, nil
}

func expectedArchiveNames(version string) []string {
	prefix := "tlsferry_" + version + "_"
	return []string{
		prefix + "darwin_amd64.tar.gz",
		prefix + "darwin_arm64.tar.gz",
		prefix + "linux_amd64.tar.gz",
		prefix + "linux_arm64.tar.gz",
		prefix + "windows_amd64.zip",
		prefix + "windows_arm64.zip",
	}
}

func validateChecksumCoverage(checksums map[string]string, expected []string) error {
	expectedSet := make(map[string]struct{}, len(expected))
	for _, name := range expected {
		expectedSet[name] = struct{}{}
		if _, ok := checksums[name]; !ok {
			return fmt.Errorf("checksums.txt is missing %s", name)
		}
	}
	for name := range checksums {
		if _, ok := expectedSet[name]; !ok {
			return fmt.Errorf("checksums.txt contains unexpected file %s", name)
		}
	}
	return nil
}

func loadBundledSource(root string) (map[string][]byte, error) {
	result := make(map[string][]byte, len(bundledPublicFiles))
	for _, name := range bundledPublicFiles {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			return nil, fmt.Errorf("read source %s: %w", name, err)
		}
		result[name] = content
	}
	return result, nil
}

func hashRegularFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("inspect archive %s: %w", filepath.Base(path), err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("archive %s is not a regular file", filepath.Base(path))
	}
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open archive %s: %w", filepath.Base(path), err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash archive %s: %w", filepath.Base(path), err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func readArchive(path string) (map[string]archiveEntry, error) {
	if strings.HasSuffix(path, ".tar.gz") {
		return readTarGzip(path)
	}
	if strings.HasSuffix(path, ".zip") {
		return readZip(path)
	}
	return nil, fmt.Errorf("unsupported archive format %q", filepath.Base(path))
}

func readTarGzip(path string) (map[string]archiveEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	entries := map[string]archiveEntry{}
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return nil, fmt.Errorf("entry %q is not a regular file", header.Name)
		}
		if err := addArchiveEntry(entries, header.Name, header.FileInfo().Mode(), header.Size, reader); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

func readZip(path string) (map[string]archiveEntry, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	entries := map[string]archiveEntry{}
	for _, file := range reader.File {
		if !file.Mode().IsRegular() {
			return nil, fmt.Errorf("entry %q is not a regular file", file.Name)
		}
		entryReader, err := file.Open()
		if err != nil {
			return nil, err
		}
		err = addArchiveEntry(entries, file.Name, file.Mode(), int64(file.UncompressedSize64), entryReader)
		closeErr := entryReader.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	return entries, nil
}

func addArchiveEntry(entries map[string]archiveEntry, name string, mode fs.FileMode, size int64, reader io.Reader) error {
	if !safeArchiveEntryName(name) {
		return fmt.Errorf("unsafe archive entry %q", name)
	}
	if _, exists := entries[name]; exists {
		return fmt.Errorf("duplicate archive entry %q", name)
	}
	if size < 0 || size > maxArchiveEntrySize {
		return fmt.Errorf("archive entry %q has invalid size %d", name, size)
	}
	content, err := io.ReadAll(io.LimitReader(reader, maxArchiveEntrySize+1))
	if err != nil {
		return fmt.Errorf("read archive entry %q: %w", name, err)
	}
	if int64(len(content)) != size {
		return fmt.Errorf("archive entry %q size mismatch", name)
	}
	entries[name] = archiveEntry{Data: content, Mode: mode}
	return nil
}

func safeArchiveEntryName(name string) bool {
	return name != "" && name != "." && !strings.ContainsAny(name, "/\\\x00") && filepath.Base(name) == name
}

func validateArchiveEntries(entries map[string]archiveEntry, binaryName string, source map[string][]byte, requireExecutable bool) ([]byte, error) {
	expected := make(map[string]struct{}, len(bundledPublicFiles)+1)
	for _, name := range bundledPublicFiles {
		expected[name] = struct{}{}
		entry, ok := entries[name]
		if !ok {
			return nil, fmt.Errorf("missing bundled file %s", name)
		}
		if !bytes.Equal(entry.Data, source[name]) {
			return nil, fmt.Errorf("bundled file %s differs from source", name)
		}
	}
	expected[binaryName] = struct{}{}
	binary, ok := entries[binaryName]
	if !ok {
		return nil, fmt.Errorf("missing binary %s", binaryName)
	}
	if len(binary.Data) == 0 {
		return nil, fmt.Errorf("binary %s is empty", binaryName)
	}
	if requireExecutable && binary.Mode.Perm()&0o111 == 0 {
		return nil, fmt.Errorf("binary %s is not executable", binaryName)
	}
	for name := range entries {
		if _, ok := expected[name]; !ok {
			return nil, fmt.Errorf("unexpected archive entry %s", name)
		}
	}
	if len(entries) != len(expected) {
		return nil, fmt.Errorf("archive has %d entries, want %d", len(entries), len(expected))
	}
	return binary.Data, nil
}

func nativeArchiveIdentity() (suffix, binaryName string, err error) {
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		return "", "", fmt.Errorf("unsupported verification architecture %s", runtime.GOARCH)
	}
	switch runtime.GOOS {
	case "darwin", "linux":
		return "_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz", "tlsferry", nil
	case "windows":
		return "_windows_" + runtime.GOARCH + ".zip", "tlsferry.exe", nil
	default:
		return "", "", fmt.Errorf("unsupported verification operating system %s", runtime.GOOS)
	}
}

func executeVersion(binary []byte, name string) (string, error) {
	directory, err := os.MkdirTemp("", "tlsferry-artifact-smoke-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(directory)
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, binary, 0o700); err != nil {
		return "", fmt.Errorf("write native release binary: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, "version").CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", errors.New("native release binary version command timed out")
	}
	if err != nil {
		return "", fmt.Errorf("run native release binary: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}
