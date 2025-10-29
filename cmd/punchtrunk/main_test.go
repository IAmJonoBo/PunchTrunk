package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestHotspotSmoke spins up a dedicated git repository and ensures hotspot
// scoring returns deterministic, non-empty results for the latest commit.
func TestHotspotSmoke(t *testing.T) {
	t.Helper()
	repo := t.TempDir()
	gitInit(t, repo)
	writeFile(t, repo, "main.go", `package main

func hello() string { return "hi" }
`)
	gitAddCommit(t, repo, "initial commit")
	// Second commit introduces churn on main.go and adds utils.go so that
	// hotspots sees both changed files.
	writeFile(t, repo, "main.go", `package main

func hello() string {
	return "hi there"
}

func newHelper() int { return 42 }
`)
	writeFile(t, repo, "utils.go", `package main

func repeat(input string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += input
	}
	return result
}
`)
	gitAddCommit(t, repo, "introduce churn")
	oldCwd := mustChdir(t, repo)
	defer func() {
		_ = os.Chdir(oldCwd)
	}()
	cfg := &Config{
		Modes:      []string{"hotspots"},
		Autofix:    "fmt",
		BaseBranch: "HEAD~1",
		Timeout:    10 * time.Second,
		SarifOut:   filepath.Join(repo, "reports", "hotspots.sarif"),
	}
	o, err := computeHotspots(context.Background(), cfg)
	if err != nil {
		t.Fatalf("computeHotspots: %v", err)
	}
	if len(o) < 2 {
		t.Fatalf("expected at least two hotspots, got %d", len(o))
	}
	if o[0].Score <= 0 {
		t.Fatalf("expected positive score, got %f", o[0].Score)
	}
	if o[0].Score < o[1].Score {
		t.Fatalf("expected scores to be sorted descending: %+v", o[:2])
	}
	files := map[string]bool{}
	for _, h := range o {
		files[h.File] = true
	}
	if !files["main.go"] || !files["utils.go"] {
		t.Fatalf("expected hotspots to include main.go and utils.go: %+v", o)
	}
}

// TestWriteSARIF ensures the SARIF writer emits valid JSON with expected rule
// metadata so downstream uploads succeed.
func TestWriteSARIF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.sarif")
	hs := []Hotspot{{File: "main.go", Churn: 10, Complexity: 1.5, Score: 4.2}}
	if err := writeSARIF(path, hs); err != nil {
		t.Fatalf("writeSARIF: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read SARIF: %v", err)
	}
	var log SarifLog
	if err := json.Unmarshal(data, &log); err != nil {
		t.Fatalf("unmarshal SARIF: %v", err)
	}
	if len(log.Runs) != 1 {
		t.Fatalf("expected 1 SARIF run, got %d", len(log.Runs))
	}
	if len(log.Runs[0].Results) != 1 {
		t.Fatalf("expected 1 SARIF result, got %d", len(log.Runs[0].Results))
	}
	if log.Runs[0].Results[0].RuleID != "hotspot" {
		t.Fatalf("unexpected rule id %s", log.Runs[0].Results[0].RuleID)
	}
	if log.Runs[0].Results[0].Locations[0].PhysicalLocation.ArtifactLocation.URI != "main.go" {
		t.Fatalf("unexpected URI %s", log.Runs[0].Results[0].Locations[0].PhysicalLocation.ArtifactLocation.URI)
	}
}

func TestEnsureEnvironmentAirgappedRequiresBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink-based PATH isolation not supported on Windows")
	}
	toolDir := prepareToolchainDir(t, false)
	t.Setenv("PATH", toolDir)
	t.Setenv("PUNCHTRUNK_AIRGAPPED", "1")
	cfg := &Config{Autofix: "fmt"}
	err := ensureEnvironment(context.Background(), cfg)
	if err == nil {
		t.Fatalf("expected error when airgapped without trunk binary")
	}
	if !strings.Contains(err.Error(), "Provide --trunk-binary") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEnsureEnvironmentAirgappedWithBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink-based PATH isolation not supported on Windows")
	}
	toolDir := prepareToolchainDir(t, true)
	t.Setenv("PATH", toolDir)
	t.Setenv("PUNCHTRUNK_AIRGAPPED", "1")
	cfg := &Config{Autofix: "fmt"}
	if err := ensureEnvironment(context.Background(), cfg); err != nil {
		t.Fatalf("ensureEnvironment: %v", err)
	}
	expected := filepath.Join(toolDir, trunkExecutableName())
	if cfg.TrunkPath != expected {
		t.Fatalf("expected trunk path %s, got %s", expected, cfg.TrunkPath)
	}
}

func TestBuildDryRunPlan(t *testing.T) {
	stubDir := t.TempDir()
	stub := makeTrunkStub(t, stubDir)
	cfg := &Config{
		Modes:          []string{"fmt", "lint"},
		Autofix:        "all",
		SarifOut:       "reports/hotspots.sarif",
		TrunkArgs:      []string{"--filter=tool:eslint"},
		TrunkConfigDir: "/tmp/trunk",
		TrunkBinary:    stub,
	}
	plan, err := buildDryRunPlan(cfg)
	if err != nil {
		t.Fatalf("buildDryRunPlan: %v", err)
	}
	if plan.Trunk.Status != "available" {
		t.Fatalf("expected trunk status available, got %s", plan.Trunk.Status)
	}
	resolvedStub, _ := filepath.Abs(stub)
	if plan.Trunk.Path != resolvedStub {
		t.Fatalf("expected trunk path %s, got %s", resolvedStub, plan.Trunk.Path)
	}
	if len(plan.Modes) != 2 {
		t.Fatalf("expected 2 modes, got %d", len(plan.Modes))
	}
	fmtMode := plan.Modes[0]
	if fmtMode.Name != "fmt" {
		t.Fatalf("expected first mode fmt, got %s", fmtMode.Name)
	}
	lintMode := plan.Modes[1]
	if lintMode.Name != "lint" {
		t.Fatalf("expected second mode lint, got %s", lintMode.Name)
	}
	if !slices.Contains(lintMode.Command, "--fix") {
		t.Fatalf("expected lint command to include --fix, got %v", lintMode.Command)
	}
}

func TestBuildDryRunPlanMissingBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	cfg := &Config{Modes: []string{"fmt"}}
	plan, err := buildDryRunPlan(cfg)
	if err != nil {
		t.Fatalf("buildDryRunPlan: %v", err)
	}
	if plan.Trunk.Status != "missing" {
		t.Fatalf("expected trunk status missing, got %s", plan.Trunk.Status)
	}
	if !plan.Trunk.AutoInstall {
		t.Fatalf("expected plan to attempt auto-install when trunk missing")
	}
}

func TestDryRunCLI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("dry-run CLI test relies on POSIX shell script stub")
	}
	root := repoRoot(t)
	binDir := t.TempDir()
	binary := filepath.Join(binDir, "punchtrunk")
	build := exec.Command("go", "build", "-o", binary, "./cmd/punchtrunk")
	build.Dir = root
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build punchtrunk: %v\n%s", err, out)
	}
	trunkDir := t.TempDir()
	trunkStub := makeTrunkStub(t, trunkDir)
	cmd := exec.Command(binary,
		"--dry-run",
		"--mode", "fmt,lint",
		"--autofix", "all",
		"--trunk-binary", trunkStub,
		"--trunk-arg=--filter=tool:eslint",
	)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dry-run command failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "--fix") {
		t.Fatalf("expected output to mention --fix, got %s", output)
	}
	if !strings.Contains(output, "No commands executed because --dry-run is enabled") {
		t.Fatalf("expected output to mention no commands executed, got %s", output)
	}
}

func TestRunHotspotsRedirectsOnReadOnlyWorkspace(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)
	writeFile(t, repo, "main.go", "package main\n\nfunc main() {}\n")
	gitAddCommit(t, repo, "initial commit")
	writeFile(t, repo, "main.go", "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n")
	gitAddCommit(t, repo, "update main")

	prev := mustChdir(t, repo)
	defer func() {
		_ = os.Chdir(prev)
	}()

	readonlyRoot := filepath.Join(repo, "readonly")
	if err := os.Mkdir(readonlyRoot, 0o555); err != nil {
		t.Fatalf("mkdir readonly: %v", err)
	}
	defer func() {
		_ = os.Chmod(readonlyRoot, 0o755)
	}()

	baseName := fmt.Sprintf("hotspots-%d.sarif", time.Now().UnixNano())
	originalOut := filepath.Join(readonlyRoot, "subdir", baseName)
	cfg := &Config{
		Modes:      []string{"hotspots"},
		BaseBranch: "HEAD~1",
		Timeout:    5 * time.Second,
		SarifOut:   originalOut,
	}

	if err := runHotspots(context.Background(), cfg); err != nil {
		t.Fatalf("runHotspots: %v", err)
	}

	expected := filepath.Join(cfg.tempDir(), "punchtrunk", "reports", baseName)
	if cfg.SarifOut != expected {
		t.Fatalf("expected SARIF path %s, got %s", expected, cfg.SarifOut)
	}
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected fallback SARIF at %s: %v", expected, err)
	}
	t.Cleanup(func() {
		_ = os.Remove(expected)
	})
}

func TestRunHotspotsUsesCustomTmpDirFallback(t *testing.T) {
	repo := t.TempDir()
	gitInit(t, repo)
	writeFile(t, repo, "main.go", "package main\n\nfunc main() {}\n")
	gitAddCommit(t, repo, "initial commit")
	writeFile(t, repo, "main.go", "package main\n\nfunc main() {\n\tprintln(\"hi\")\n}\n")
	gitAddCommit(t, repo, "update main")

	prev := mustChdir(t, repo)
	defer func() {
		_ = os.Chdir(prev)
	}()

	readonlyRoot := filepath.Join(repo, "readonly")
	if err := os.Mkdir(readonlyRoot, 0o555); err != nil {
		t.Fatalf("mkdir readonly: %v", err)
	}
	defer func() {
		_ = os.Chmod(readonlyRoot, 0o755)
	}()

	customTmp := filepath.Join(repo, "custom-tmp")

	baseName := fmt.Sprintf("hotspots-%d.sarif", time.Now().UnixNano())
	originalOut := filepath.Join(readonlyRoot, "subdir", baseName)
	cfg := &Config{
		Modes:      []string{"hotspots"},
		BaseBranch: "HEAD~1",
		Timeout:    5 * time.Second,
		SarifOut:   originalOut,
		TmpDir:     customTmp,
	}

	if err := runHotspots(context.Background(), cfg); err != nil {
		t.Fatalf("runHotspots: %v", err)
	}

	resDir := filepath.Join(customTmp, "punchtrunk", "reports", baseName)
	if cfg.SarifOut != resDir {
		t.Fatalf("expected SARIF path %s, got %s", resDir, cfg.SarifOut)
	}
	if _, err := os.Stat(resDir); err != nil {
		t.Fatalf("expected fallback SARIF at %s: %v", resDir, err)
	}
	t.Cleanup(func() {
		_ = os.Remove(resDir)
	})
}

func TestConfigResolveTmpDirRelative(t *testing.T) {
	cwd := t.TempDir()
	prev := mustChdir(t, cwd)
	defer func() {
		_ = os.Chdir(prev)
	}()

	cfg := &Config{TmpDir: filepath.Join("relative", "tmp")}
	resolved, err := cfg.resolveTmpDir()
	if err != nil {
		t.Fatalf("resolveTmpDir: %v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	expected := filepath.Join(wd, "relative", "tmp")
	if resolved != expected {
		t.Fatalf("expected resolved tmp dir %s, got %s", expected, resolved)
	}
	if _, err := os.Stat(expected); err != nil {
		t.Fatalf("expected tmp dir to exist: %v", err)
	}
}

func TestOfflineBundleSupportsAirgappedHotspots(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("offline bundle packaging not validated on Windows")
	}
	if _, err := exec.LookPath("tar"); err != nil {
		t.Skipf("tar not available: %v", err)
	}

	root := repoRoot(t)

	binDir := t.TempDir()
	punchBinary := filepath.Join(binDir, "punchtrunk")
	build := exec.Command("go", "build", "-o", punchBinary, "./cmd/punchtrunk")
	build.Dir = root
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build punchtrunk: %v\n%s", err, out)
	}

	stubDir := t.TempDir()
	trunkStub := filepath.Join(stubDir, trunkExecutableName())
	stub := "#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"${1:-}\" == \"--version\" ]]; then\n  echo \"stub trunk version 0.0.0\"\n  exit 0\nfi\nexit 0\n"
	if err := os.WriteFile(trunkStub, []byte(stub), 0o755); err != nil {
		t.Fatalf("write trunk stub: %v", err)
	}

	cacheDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cacheDir, "tool.lock"), []byte("demo"), 0o644); err != nil {
		t.Fatalf("write cache stub: %v", err)
	}

	script := filepath.Join(root, "scripts", "build-offline-bundle.sh")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("bundle script missing: %v", err)
	}

	outDir := t.TempDir()
	bundleName := "test-offline-bundle.tgz"
	cmd := exec.Command("bash", script,
		"--punchtrunk-binary", punchBinary,
		"--trunk-binary", trunkStub,
		"--cache-dir", cacheDir,
		"--config-dir", filepath.Join(root, ".trunk"),
		"--output-dir", outDir,
		"--bundle-name", bundleName,
		"--force",
	)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build offline bundle: %v\n%s", err, out)
	}

	bundlePath := filepath.Join(outDir, bundleName)
	if _, err := os.Stat(bundlePath); err != nil {
		t.Fatalf("bundle not created: %v", err)
	}
	if _, err := os.Stat(bundlePath + ".sha256"); err != nil {
		t.Fatalf("bundle checksum missing: %v", err)
	}

	extractDir := t.TempDir()
	untar := exec.Command("tar", "-xzf", bundlePath, "-C", extractDir)
	if out, err := untar.CombinedOutput(); err != nil {
		t.Fatalf("untar bundle: %v\n%s", err, out)
	}
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		t.Fatalf("read extract dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected single bundle root, got %d", len(entries))
	}
	bundleRoot := filepath.Join(extractDir, entries[0].Name())
	bundlePunch := filepath.Join(bundleRoot, "bin", "punchtrunk")
	bundleTrunk := filepath.Join(bundleRoot, "trunk", "bin", trunkExecutableName())
	manifest := filepath.Join(bundleRoot, "manifest.json")
	checksums := filepath.Join(bundleRoot, "checksums.txt")
	if _, err := os.Stat(bundlePunch); err != nil {
		t.Fatalf("bundle punchtrunk missing: %v", err)
	}
	if _, err := os.Stat(bundleTrunk); err != nil {
		t.Fatalf("bundle trunk missing: %v", err)
	}
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	contents, err := os.ReadFile(checksums)
	if err != nil {
		t.Fatalf("read checksums: %v", err)
	}
	if !strings.Contains(string(contents), "bin/punchtrunk") {
		t.Fatalf("checksums missing punchtrunk entry: %s", contents)
	}

	repo := t.TempDir()
	gitInit(t, repo)
	writeFile(t, repo, "main.go", "package main\n\nfunc main() {}\n")
	gitAddCommit(t, repo, "initial commit")
	writeFile(t, repo, "main.go", "package main\n\nfunc main() {\n    println(\"hi\")\n}\n")
	gitAddCommit(t, repo, "update main")

	cmd = exec.Command(bundlePunch,
		"--mode", "hotspots",
		"--base-branch", "HEAD~1",
		"--trunk-binary", bundleTrunk,
		"--timeout", "30",
	)
	cmd.Dir = repo
	cmd.Env = append(os.Environ(),
		"PUNCHTRUNK_AIRGAPPED=1",
		fmt.Sprintf("PUNCHTRUNK_TRUNK_BINARY=%s", bundleTrunk),
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bundle punchtrunk execution failed: %v\n%s", err, out)
	}

	sarifPath := filepath.Join(repo, "reports", "hotspots.sarif")
	if _, err := os.Stat(sarifPath); err != nil {
		t.Fatalf("expected SARIF output: %v", err)
	}
}

func TestDiagnoseAirgapHappyPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("diagnostic shell script relies on POSIX sh")
	}
	t.Setenv("PUNCHTRUNK_AIRGAPPED", "1")
	t.Setenv("PUNCHTRUNK_TRUNK_BINARY", "")
	reportsDir := t.TempDir()
	trunkPath := filepath.Join(reportsDir, "trunk")
	script := "#!/bin/sh\necho trunk version 1.2.3\n"
	if err := os.WriteFile(trunkPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write trunk script: %v", err)
	}
	sarifDir := filepath.Join(reportsDir, "reports")
	if err := os.MkdirAll(sarifDir, 0o755); err != nil {
		t.Fatalf("mkdir reports: %v", err)
	}
	cfg := &Config{
		TrunkBinary: trunkPath,
		SarifOut:    filepath.Join(sarifDir, "hotspots.sarif"),
	}
	report := diagnoseAirgap(cfg)
	if !report.Airgapped {
		t.Fatalf("expected airgapped true got false")
	}
	if report.Summary.Error != 0 {
		t.Fatalf("expected no errors: %+v", report.Summary)
	}
	if report.Summary.OK == 0 {
		t.Fatalf("expected OK checks: %+v", report.Summary)
	}
	foundTrunk := false
	for _, c := range report.Checks {
		if c.Name == "trunk_binary" {
			foundTrunk = true
			if c.Status != diagnoseStatusOK {
				t.Fatalf("expected trunk check ok: %+v", c)
			}
		}
	}
	if !foundTrunk {
		t.Fatalf("expected trunk check present: %+v", report.Checks)
	}
}

func TestDiagnoseAirgapDetectsMissingTrunk(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("diagnostic shell script relies on POSIX sh")
	}
	t.Setenv("PUNCHTRUNK_AIRGAPPED", "1")
	newHome := t.TempDir()
	t.Setenv("HOME", newHome)
	reportsDir := filepath.Join(newHome, "reports")
	if err := os.MkdirAll(reportsDir, 0o755); err != nil {
		t.Fatalf("mkdir reports: %v", err)
	}
	cfg := &Config{
		SarifOut: filepath.Join(reportsDir, "hotspots.sarif"),
	}
	report := diagnoseAirgap(cfg)
	if report.Summary.Error == 0 {
		t.Fatalf("expected errors: %+v", report.Summary)
	}
	found := false
	for _, c := range report.Checks {
		if c.Name == "trunk_binary" {
			found = true
			if c.Status != diagnoseStatusError {
				t.Fatalf("expected trunk check error: %+v", c)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected trunk check present: %+v", report.Checks)
	}
}

func TestEventLoggerJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := newEventLogger(&buf, true)
	logger.Event("info", "mode.start", LogFields{"mode": "fmt", "duration_ms": 42})
	logger.Infof("plain message")
	dec := json.NewDecoder(&buf)
	first := map[string]any{}
	if err := dec.Decode(&first); err != nil {
		t.Fatalf("decode first event: %v", err)
	}
	if first["event"] != "mode.start" {
		t.Fatalf("expected event mode.start, got %v", first["event"])
	}
	if first["mode"] != "fmt" {
		t.Fatalf("expected mode fmt, got %v", first["mode"])
	}
	if _, ok := first["ts"].(string); !ok {
		t.Fatalf("expected timestamp string, got %T", first["ts"])
	}
	second := map[string]any{}
	if err := dec.Decode(&second); err != nil {
		t.Fatalf("decode second event: %v", err)
	}
	if second["message"] != "plain message" {
		t.Fatalf("unexpected second message: %v", second["message"])
	}
	if second["level"] != "info" {
		t.Fatalf("unexpected level: %v", second["level"])
	}
}

func TestConfigLoggerReuse(t *testing.T) {
	cfg := &Config{JSONLogs: true}
	logger := cfg.log()
	if logger == nil {
		t.Fatalf("expected logger instance")
	}
	if logger != cfg.log() {
		t.Fatalf("expected cached logger instance")
	}
}

func setupTestFlags(t *testing.T, args []string) {
	t.Helper()
	if len(args) == 0 {
		t.Fatalf("args must include binary name")
	}
	oldArgs := os.Args
	oldCommandLine := flag.CommandLine
	t.Cleanup(func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	})
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	flag.CommandLine = fs
	os.Args = args
}

func TestParseFlagsDefaultsUseEnv(t *testing.T) {
	setupTestFlags(t, []string{"punchtrunk"})
	tmpDir := t.TempDir()
	t.Setenv("PUNCHTRUNK_JSON_LOGS", "true")
	t.Setenv("PUNCHTRUNK_TMP_DIR", tmpDir)
	t.Setenv("PUNCHTRUNK_TRUNK_BINARY", "/custom/trunk")

	cfg := parseFlags()

	if !cfg.JSONLogs {
		t.Fatalf("expected JSON logs enabled via env")
	}
	if cfg.TmpDir != tmpDir {
		t.Fatalf("expected tmp dir %s, got %s", tmpDir, cfg.TmpDir)
	}
	if cfg.TrunkBinary != "/custom/trunk" {
		t.Fatalf("expected trunk binary from env, got %s", cfg.TrunkBinary)
	}
	wantModes := []string{"fmt", "lint", "hotspots"}
	if !slices.Equal(cfg.Modes, wantModes) {
		t.Fatalf("unexpected default modes: %v", cfg.Modes)
	}
	if cfg.Autofix != "fmt" {
		t.Fatalf("expected default autofix fmt, got %s", cfg.Autofix)
	}
	if cfg.Timeout != 900*time.Second {
		t.Fatalf("expected default timeout of 900s, got %s", cfg.Timeout)
	}
}

func TestParseFlagsOverrides(t *testing.T) {
	args := []string{
		"punchtrunk",
		"--mode", "fmt,lint",
		"--autofix", "ALL",
		"--base-branch", "feature/foo",
		"--timeout", "0",
		"--dry-run",
		"--verbose",
		"--json-logs",
		"--trunk-config-dir", "/tmp/conf",
		"--tmp-dir", "./tmp/out",
		"--sarif-out", "./reports/out.sarif",
		"--trunk-binary", "/opt/trunk/bin/trunk",
		"--trunk-arg=--filter=tool:eslint",
		"--trunk-arg=--foo",
	}
	setupTestFlags(t, args)

	cfg := parseFlags()

	wantModes := []string{"fmt", "lint"}
	if !slices.Equal(cfg.Modes, wantModes) {
		t.Fatalf("unexpected modes: %v", cfg.Modes)
	}
	if cfg.Autofix != "all" {
		t.Fatalf("expected autofix lowered to all, got %s", cfg.Autofix)
	}
	if cfg.BaseBranch != "feature/foo" {
		t.Fatalf("unexpected base branch: %s", cfg.BaseBranch)
	}
	if cfg.Timeout != 0 {
		t.Fatalf("expected timeout disabled, got %s", cfg.Timeout)
	}
	if !cfg.DryRun {
		t.Fatalf("expected dry-run enabled")
	}
	if !cfg.JSONLogs {
		t.Fatalf("expected json logs enabled")
	}
	if !cfg.Verbose {
		t.Fatalf("expected verbose enabled")
	}
	if cfg.TrunkConfigDir != "/tmp/conf" {
		t.Fatalf("unexpected trunk config dir: %s", cfg.TrunkConfigDir)
	}
	if cfg.TmpDir != "./tmp/out" {
		t.Fatalf("unexpected tmp dir: %s", cfg.TmpDir)
	}
	if cfg.SarifOut != "reports/out.sarif" {
		t.Fatalf("expected cleaned sarif path, got %s", cfg.SarifOut)
	}
	if cfg.TrunkBinary != "/opt/trunk/bin/trunk" {
		t.Fatalf("unexpected trunk binary: %s", cfg.TrunkBinary)
	}
	gotArgs := []string(cfg.TrunkArgs)
	wantArgs := []string{"--filter=tool:eslint", "--foo"}
	if !slices.Equal(gotArgs, wantArgs) {
		t.Fatalf("unexpected trunk args: %v", gotArgs)
	}
}

func TestRunTrunkFmtAppliesEnvAndArgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stubs not supported on Windows in this test")
	}
	prev := mustChdir(t, t.TempDir())
	defer func() {
		_ = os.Chdir(prev)
	}()

	stubDir := t.TempDir()
	stubPath := filepath.Join(stubDir, trunkExecutableName())
	script := "#!/bin/sh\nset -eu\nout=${PUNCHTRUNK_TEST_OUTPUT:?missing}\nprintf '%s\\n' \"$*\" > \"$out\"\nprintf 'TRUNK_CONFIG_DIR=%s\\n' \"${TRUNK_CONFIG_DIR:-}\" >> \"$out\"\n"
	if err := os.WriteFile(stubPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	outFile := filepath.Join(t.TempDir(), "fmt.txt")
	t.Setenv("PUNCHTRUNK_TEST_OUTPUT", outFile)

	cfg := &Config{
		TrunkPath:      stubPath,
		TrunkArgs:      []string{"--filter=demo"},
		TrunkConfigDir: "/tmp/trunk-config",
	}
	cfg.logger = newEventLogger(io.Discard, false)

	if err := runTrunkFmt(context.Background(), cfg); err != nil {
		t.Fatalf("runTrunkFmt: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read stub output: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected two lines of output, got %d: %q", len(lines), lines)
	}
	if !strings.Contains(lines[0], "fmt") || !strings.Contains(lines[0], "--filter=demo") {
		t.Fatalf("unexpected command line: %s", lines[0])
	}
	if lines[1] != "TRUNK_CONFIG_DIR=/tmp/trunk-config" {
		t.Fatalf("expected TRUNK_CONFIG_DIR line, got %s", lines[1])
	}
}

func TestRunTrunkCheckSetsExitErr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stubs not supported on Windows in this test")
	}
	prev := mustChdir(t, t.TempDir())
	defer func() {
		_ = os.Chdir(prev)
	}()

	stubDir := t.TempDir()
	stubPath := filepath.Join(stubDir, trunkExecutableName())
	if err := os.WriteFile(stubPath, []byte("#!/bin/sh\nexit 7\n"), 0o755); err != nil {
		t.Fatalf("write stub: %v", err)
	}

	cfg := &Config{
		TrunkPath: stubPath,
		Autofix:   "all",
	}
	cfg.logger = newEventLogger(io.Discard, false)

	exitErr = nil
	t.Cleanup(func() {
		exitErr = nil
	})

	err := runTrunkCheck(context.Background(), cfg)
	if err == nil {
		t.Fatalf("expected error from failing stub")
	}
	if exitErr != err {
		t.Fatalf("expected exitErr to capture runTrunkCheck error")
	}
}

func gitInit(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.name", "PunchTrunk Test")
	runGit(t, dir, "config", "user.email", "punchtrunk@example.com")
}

func gitAddCommit(t *testing.T, dir, message string) {
	t.Helper()
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", message)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=PunchTrunk Test",
		"GIT_AUTHOR_EMAIL=punchtrunk@example.com",
		"GIT_COMMITTER_NAME=PunchTrunk Test",
		"GIT_COMMITTER_EMAIL=punchtrunk@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func writeFile(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func mustChdir(t *testing.T, dir string) string {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	return prev
}

// TestRoughComplexity validates the complexity heuristic for various file types.
func TestRoughComplexity(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantMin float64
		wantMax float64
	}{
		{
			name:    "simple go file",
			content: "package main\n\nfunc main() {\n}\n",
			wantMin: 1.0,
			wantMax: 3.0,
		},
		{
			name:    "complex go file",
			content: "package main\n\nfunc complex() {\n  x := 1\n  y := 2\n  z := x + y\n  return z\n}\n",
			wantMin: 2.0,
			wantMax: 5.0,
		},
		{
			name:    "empty file",
			content: "",
			wantMin: 0.0,
			wantMax: 0.0,
		},
		{
			name:    "single line",
			content: "package main",
			wantMin: 1.0,
			wantMax: 3.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.go")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("writeFile: %v", err)
			}

			complexity, err := roughComplexity(path)
			if err != nil {
				t.Fatalf("roughComplexity: %v", err)
			}

			if complexity < tt.wantMin || complexity > tt.wantMax {
				t.Errorf("complexity = %f, want between %f and %f", complexity, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestDetectCompetingToolConfigsBlackValidation(t *testing.T) {
	dir := t.TempDir()
	prev := mustChdir(t, dir)
	defer func() {
		_ = os.Chdir(prev)
	}()

	// pyproject without [tool.black] should not trigger a warning.
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\nname = \"demo\"\n"), 0o644); err != nil {
		t.Fatalf("write pyproject: %v", err)
	}
	msgs := detectCompetingToolConfigs("fmt")
	for _, msg := range msgs {
		if strings.Contains(msg, "Black") {
			t.Fatalf("expected no Black warning, got %q", msg)
		}
	}

	// Adding [tool.black] should surface the warning.
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[tool.black]\nline-length = 88\n"), 0o644); err != nil {
		t.Fatalf("rewrite pyproject: %v", err)
	}
	msgs = detectCompetingToolConfigs("fmt")
	found := false
	for _, msg := range msgs {
		if strings.Contains(msg, "Black") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected Black warning after adding [tool.black], got %+v", msgs)
	}
}

func prepareToolchainDir(t *testing.T, includeTrunk bool) string {
	t.Helper()
	dir := t.TempDir()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available in PATH")
	}
	if err := os.Symlink(gitPath, filepath.Join(dir, "git")); err != nil {
		t.Fatalf("symlink git: %v", err)
	}
	if includeTrunk {
		trunkPath, err := exec.LookPath("trunk")
		if err != nil {
			t.Skip("trunk not installed; install locally to run airgap tests")
		}
		if err := os.Symlink(trunkPath, filepath.Join(dir, trunkExecutableName())); err != nil {
			t.Fatalf("symlink trunk: %v", err)
		}
	}
	return dir
}

func makeTrunkStub(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir trunk stub dir: %v", err)
	}
	stub := filepath.Join(dir, trunkExecutableName())
	script := "#!/bin/sh\nexit 0\n"
	if runtime.GOOS == "windows" {
		script = "@echo off\r\nexit /B 0\r\n"
	}
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatalf("write trunk stub: %v", err)
	}
	return stub
}

// TestMeanStd validates statistical helper functions.
func TestMeanStd(t *testing.T) {
	tests := []struct {
		name     string
		vals     []float64
		wantMean float64
		wantStd  float64
	}{
		{
			name:     "empty",
			vals:     []float64{},
			wantMean: 0.0,
			wantStd:  0.0,
		},
		{
			name:     "single value",
			vals:     []float64{5.0},
			wantMean: 5.0,
			wantStd:  0.0,
		},
		{
			name:     "uniform values",
			vals:     []float64{3.0, 3.0, 3.0},
			wantMean: 3.0,
			wantStd:  0.0,
		},
		{
			name:     "varied values",
			vals:     []float64{1.0, 2.0, 3.0, 4.0, 5.0},
			wantMean: 3.0,
			wantStd:  1.4142, // approximately sqrt(2)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mean, std := meanStd(tt.vals)

			if mean != tt.wantMean {
				t.Errorf("mean = %f, want %f", mean, tt.wantMean)
			}

			// Allow some tolerance for floating point
			if tt.wantStd > 0 && (std < tt.wantStd-0.01 || std > tt.wantStd+0.01) {
				t.Errorf("std = %f, want %f (±0.01)", std, tt.wantStd)
			} else if tt.wantStd == 0 && std != 0 {
				t.Errorf("std = %f, want %f", std, tt.wantStd)
			}
		})
	}
}

// TestSplitCSV validates CSV parsing helper.
func TestSplitCSV(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"fmt,lint,hotspots", []string{"fmt", "lint", "hotspots"}},
		{"fmt, lint, hotspots", []string{"fmt", "lint", "hotspots"}},
		{"fmt", []string{"fmt"}},
		{"", []string{}},
		{"  fmt  ,  lint  ", []string{"fmt", "lint"}},
		{"fmt,,lint", []string{"fmt", "lint"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitCSV(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestAtoiSafe validates safe integer parsing.
func TestAtoiSafe(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"42", 42},
		{"0", 0},
		{"-5", -5},
		{"invalid", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := atoiSafe(tt.input)
			if got != tt.want {
				t.Errorf("atoiSafe(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
