package main

// PunchTrunk: a thin wrapper around `trunk` with hotspot SARIF.
// Goals: safe defaults, ephemeral-friendly, no bespoke linter parsing.
//
// Notes:
// - We rely on Trunk for tool orchestration. See: https://docs.trunk.io/
// - Hotspots use recent git churn + crude complexity proxy.
// - SARIF generated: file-level "note" results for hotspots.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Version is set at build time via -ldflags.
var Version = "dev"

type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

type Config struct {
	Modes          []string
	Autofix        string
	BaseBranch     string
	MaxProcs       int
	Timeout        time.Duration
	SarifOut       string
	Verbose        bool
	ShowVersion    bool
	TrunkPath      string
	TrunkConfigDir string
	TrunkArgs      []string
	TrunkBinary    string
}

func main() {
	cfg := parseFlags()

	if cfg.ShowVersion {
		fmt.Printf("PunchTrunk version %s\n", Version)
		return
	}

	if cfg.MaxProcs <= 0 {
		cfg.MaxProcs = runtime.NumCPU()
	}
	runtime.GOMAXPROCS(cfg.MaxProcs)

	ctx := context.Background()
	var cancel context.CancelFunc
	if cfg.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	needsEnvironment := false
	for _, raw := range cfg.Modes {
		mode := strings.TrimSpace(strings.ToLower(raw))
		if mode == "" {
			continue
		}
		if mode != "diagnose-airgap" {
			needsEnvironment = true
			break
		}
	}

	if needsEnvironment {
		if err := ensureEnvironment(ctx, cfg); err != nil {
			log.Fatalf("environment setup failed: %v", err)
		}
	}

	for _, raw := range cfg.Modes {
		mode := strings.TrimSpace(strings.ToLower(raw))
		if mode == "" {
			continue
		}
		var err error
		switch mode {
		case "fmt":
			err = runTrunkFmt(ctx, cfg)
		case "lint":
			err = runTrunkCheck(ctx, cfg)
		case "hotspots":
			err = runHotspots(ctx, cfg)
		case "diagnose-airgap":
			err = runDiagnoseAirgap(ctx, cfg)
		default:
			if cfg.Verbose {
				log.Printf("Skipping unknown mode %q", raw)
			}
			continue
		}
		if err != nil {
			if mode == "hotspots" {
				log.Printf("WARN: %s failed: %v", mode, err)
				continue
			}
			log.Fatalf("ERROR: %s failed: %v", mode, err)
		}
	}

	if exitErr != nil {
		os.Exit(1)
	}
}

// Global to capture trunk check exit for CI policy.
var exitErr error

var (
	conflictMu           sync.Mutex
	seenConflictMessages = map[string]struct{}{}
	conflictGuidanceOnce sync.Once
)

func parseFlags() *Config {
	var modes string
	var base string
	var maxProcs int
	var timeoutSec int
	var sarifOut string
	var verbose bool
	var autofix string
	var version bool
	var trunkConfigDir string
	var trunkBinary string
	var trunkArgs multiFlag
	flag.StringVar(&modes, "mode", "fmt,lint,hotspots", "Comma-separated phases: fmt,lint,hotspots")
	flag.StringVar(&autofix, "autofix", "fmt", "Autofix scope: none|fmt|lint|all")
	flag.StringVar(&base, "base-branch", "origin/main", "Base branch for change detection")
	flag.IntVar(&maxProcs, "max-procs", 0, "Parallelism cap (0 = CPU cores)")
	flag.IntVar(&timeoutSec, "timeout", 900, "Overall timeout in seconds (0 to disable)")
	flag.StringVar(&sarifOut, "sarif-out", "reports/hotspots.sarif", "SARIF output path for hotspots")
	flag.BoolVar(&verbose, "verbose", false, "Verbose logs")
	flag.BoolVar(&version, "version", false, "Show version and exit")
	flag.StringVar(&trunkConfigDir, "trunk-config-dir", "", "Override Trunk config directory (defaults to repo autodetect)")
	flag.StringVar(&trunkBinary, "trunk-binary", "", "Explicit path to trunk executable (for airgapped runners)")
	flag.Var(&trunkArgs, "trunk-arg", "Additional argument to pass to trunk CLI (repeatable)")
	flag.Parse()

	envTrunkBinary := os.Getenv("PUNCHTRUNK_TRUNK_BINARY")
	if trunkBinary == "" && envTrunkBinary != "" {
		trunkBinary = envTrunkBinary
	}

	modeList := splitCSV(modes)
	if len(modeList) == 0 {
		modeList = []string{"fmt", "lint", "hotspots"}
	}

	timeout := time.Duration(timeoutSec) * time.Second
	if timeoutSec <= 0 {
		timeout = 0
	}

	return &Config{
		Modes:          modeList,
		Autofix:        strings.ToLower(strings.TrimSpace(autofix)),
		BaseBranch:     base,
		MaxProcs:       maxProcs,
		Timeout:        timeout,
		SarifOut:       filepath.Clean(sarifOut),
		Verbose:        verbose,
		ShowVersion:    version,
		TrunkConfigDir: trunkConfigDir,
		TrunkArgs:      trunkArgs,
		TrunkBinary:    trunkBinary,
	}
}

func (cfg *Config) trunkBinary() string {
	if cfg != nil && cfg.TrunkPath != "" {
		return cfg.TrunkPath
	}
	return "trunk"
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func runTrunkFmt(ctx context.Context, cfg *Config) error {
	args := append([]string{"fmt"}, cfg.TrunkArgs...)
	cmd := exec.CommandContext(ctx, cfg.trunkBinary(), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	applyTrunkCommandEnv(cmd, cfg)
	maybeWarnCompetingTools("fmt", cfg)
	if cfg.Verbose {
		log.Printf("Running: %s %s", cfg.trunkBinary(), strings.Join(args, " "))
	}
	return cmd.Run()
}

func runTrunkCheck(ctx context.Context, cfg *Config) error {
	args := []string{"check"}
	// Decide autofix scope
	switch cfg.Autofix {
	case "all":
		args = append(args, "--fix")
	case "lint":
		// Trunk doesn't have "lint-only fix", so we still pass --fix.
		// Users should configure which linters have fix enabled in trunk.yaml.
		args = append(args, "--fix")
	case "fmt":
		// Default path: we already ran fmt() so here we avoid --fix.
	case "none":
		// no-op
	default:
		// unknown -> none
	}
	args = append(args, cfg.TrunkArgs...)
	// Let trunk decide changed files via hold-the-line; base branch is read from trunk.yaml.
	cmd := exec.CommandContext(ctx, cfg.trunkBinary(), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	applyTrunkCommandEnv(cmd, cfg)
	maybeWarnCompetingTools("lint", cfg)
	if cfg.Verbose {
		log.Printf("Running: %s %s", cfg.trunkBinary(), strings.Join(args, " "))
	}
	err := cmd.Run()
	if err != nil {
		exitErr = err
	}
	return err
}

func runHotspots(ctx context.Context, cfg *Config) error {
	hs, err := computeHotspots(ctx, cfg)
	if err != nil {
		return err
	}
	if cfg.SarifOut == "" {
		if cfg.Verbose {
			log.Printf("Hotspots computed (%d results) but SARIF output path is empty", len(hs))
		}
		return nil
	}
	dir := filepath.Dir(cfg.SarifOut)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fallback, ok := sarifFallbackPath(cfg.SarifOut, err)
			if !ok {
				return fmt.Errorf("create SARIF directory %s: %w", dir, err)
			}
			log.Printf("INFO: unable to create SARIF directory %s: %v; writing to %s instead", dir, err, fallback)
			cfg.SarifOut = fallback
			dir = filepath.Dir(cfg.SarifOut)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("create fallback SARIF directory %s: %w", dir, err)
			}
		}
	}
	if err := writeSARIF(cfg.SarifOut, hs); err != nil {
		return err
	}
	if cfg.Verbose {
		log.Printf("Wrote SARIF to %s (%d results)", cfg.SarifOut, len(hs))
	}
	return nil
}

func sarifFallbackPath(current string, mkdirErr error) (string, bool) {
	if current == "" {
		return "", false
	}
	if !isPermissionOrReadOnly(mkdirErr) {
		return "", false
	}
	fallbackDir := filepath.Join(os.TempDir(), "punchtrunk", "reports")
	return filepath.Join(fallbackDir, filepath.Base(current)), true
}

func isPermissionOrReadOnly(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrPermission) {
		return true
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		if errors.Is(pathErr.Err, syscall.EROFS) || errors.Is(pathErr.Err, syscall.EPERM) {
			return true
		}
	}
	return strings.Contains(strings.ToLower(err.Error()), "read-only")
}

func applyTrunkCommandEnv(cmd *exec.Cmd, cfg *Config) {
	if cmd == nil {
		return
	}
	env := os.Environ()
	if cfg != nil && cfg.TrunkConfigDir != "" {
		env = append(env, fmt.Sprintf("TRUNK_CONFIG_DIR=%s", cfg.TrunkConfigDir))
	}
	cmd.Env = env
}

func maybeWarnCompetingTools(mode string, _ *Config) {
	conflicts := detectCompetingToolConfigs(mode)
	if len(conflicts) == 0 {
		return
	}
	for _, msg := range conflicts {
		conflictMu.Lock()
		if _, ok := seenConflictMessages[msg]; !ok {
			log.Printf("INFO: %s", msg)
			seenConflictMessages[msg] = struct{}{}
			conflictGuidanceOnce.Do(func() {
				log.Printf("INFO: Use --trunk-config-dir to point at the desired Trunk config or repeat --trunk-arg to forward filters that avoid tool overlap.")
			})
		}
		conflictMu.Unlock()
	}
}

func detectCompetingToolConfigs(mode string) []string {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	type def struct {
		Tool     string
		Files    []string
		Advice   string
		Validate func(path string) bool
	}
	var defs []def
	switch mode {
	case "fmt":
		defs = []def{
			{Tool: "Prettier", Files: []string{".prettierrc", ".prettierrc.json", ".prettierrc.yml", ".prettierrc.yaml", ".prettierrc.js", ".prettierrc.cjs", "prettier.config.js", "prettier.config.cjs"}, Advice: "Detected formatting config; ensure Trunk formatters and Prettier do not both rewrite the same files."},
			{Tool: "Black", Files: []string{"pyproject.toml", "black.toml"}, Advice: "Detected Python formatting config; coordinate with Trunk's Python formatters or scope them via --trunk-arg.", Validate: func(path string) bool {
				if !strings.HasSuffix(path, "pyproject.toml") {
					return true
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return false
				}
				content := strings.ToLower(string(data))
				return strings.Contains(content, "[tool.black]")
			}},
			{Tool: "clang-format", Files: []string{".clang-format"}, Advice: "Detected clang-format configuration; align Trunk's C/C++ formatters to avoid double application."},
			{Tool: "SwiftFormat", Files: []string{".swiftformat"}, Advice: "Detected Swift formatting config; limit Trunk formatters if SwiftFormat already runs in CI."},
		}
	case "lint":
		defs = []def{
			{Tool: "ESLint", Files: []string{".eslintrc", ".eslintrc.json", ".eslintrc.js", ".eslintrc.cjs", ".eslint.config.js"}, Advice: "Detected ESLint config; coordinate with Trunk lint execution to avoid duplicate diagnostics."},
			{Tool: "Stylelint", Files: []string{".stylelintrc", ".stylelintrc.json", ".stylelintrc.yaml", ".stylelintrc.yml"}, Advice: "Detected Stylelint config; ensure Trunk lint definitions do not conflict."},
			{Tool: "Pylint/Flake8", Files: []string{".pylintrc", ".flake8"}, Advice: "Detected Python linter config; configure Trunk accordingly or disable redundant runners."},
			{Tool: "Rubocop", Files: []string{".rubocop.yml"}, Advice: "Detected Rubocop config; avoid double-running Ruby lint via both Trunk and native tooling."},
		}
	default:
		return nil
	}
	var messages []string
	for _, d := range defs {
		seen := map[string]struct{}{}
		var hits []string
		for _, rel := range d.Files {
			if rel == "" {
				continue
			}
			path := filepath.Join(cwd, rel)
			if _, err := os.Stat(path); err == nil {
				if d.Validate != nil && !d.Validate(path) {
					continue
				}
				if _, ok := seen[rel]; !ok {
					seen[rel] = struct{}{}
					hits = append(hits, rel)
				}
			}
		}
		if len(hits) == 0 {
			continue
		}
		messages = append(messages, fmt.Sprintf("Detected %s configuration (%s). %s", d.Tool, strings.Join(hits, ", "), d.Advice))
	}
	return messages
}

type Hotspot struct {
	File       string
	Churn      int
	Complexity float64
	Score      float64
}

const (
	diagnoseStatusOK    = "ok"
	diagnoseStatusWarn  = "warn"
	diagnoseStatusError = "error"
)

type DiagnoseCheck struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	Message        string `json:"message"`
	Recommendation string `json:"recommendation,omitempty"`
}

type DiagnoseSummary struct {
	Total int `json:"total"`
	OK    int `json:"ok"`
	Warn  int `json:"warn"`
	Error int `json:"error"`
}

type DiagnoseReport struct {
	Timestamp string          `json:"timestamp"`
	Airgapped bool            `json:"airgapped"`
	SarifOut  string          `json:"sarif_out"`
	Checks    []DiagnoseCheck `json:"checks"`
	Summary   DiagnoseSummary `json:"summary"`
}

func ensureEnvironment(ctx context.Context, cfg *Config) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is required: %w", err)
	}

	if cfg.TrunkConfigDir != "" {
		abs, err := filepath.Abs(cfg.TrunkConfigDir)
		if err != nil {
			return fmt.Errorf("resolve trunk-config-dir: %w", err)
		}
		info, statErr := os.Stat(abs)
		if statErr != nil {
			return fmt.Errorf("trunk-config-dir %s: %w", abs, statErr)
		}
		if !info.IsDir() {
			return fmt.Errorf("trunk-config-dir %s is not a directory", abs)
		}
		cfg.TrunkConfigDir = abs
		if _, err := os.Stat(filepath.Join(abs, "trunk.yaml")); errors.Is(err, os.ErrNotExist) {
			if cfg.Verbose {
				log.Printf("WARN: trunk-config-dir %s does not contain trunk.yaml; trunk will rely on discovery", abs)
			}
		} else if err != nil {
			return fmt.Errorf("trunk-config-dir %s: %w", abs, err)
		}
	}

	if cfg.TrunkBinary != "" {
		resolved, err := resolveTrunkBinary(cfg.TrunkBinary)
		if err != nil {
			return fmt.Errorf("trunk-binary validation: %w", err)
		}
		cfg.TrunkPath = resolved
		if cfg.Verbose {
			log.Printf("Using user-supplied trunk binary: %s", resolved)
		}
		return nil
	}

	trunkPath, err := ensureTrunk(ctx, cfg)
	if err != nil {
		return err
	}
	cfg.TrunkPath = trunkPath
	return nil
}

func runDiagnoseAirgap(ctx context.Context, cfg *Config) error {
	report := diagnoseAirgap(cfg)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal diagnostics: %w", err)
	}
	fmt.Println(string(data))
	if report.Summary.Error > 0 {
		return fmt.Errorf("diagnostics found %d blocking issue(s)", report.Summary.Error)
	}
	return nil
}

func diagnoseAirgap(cfg *Config) DiagnoseReport {
	if cfg == nil {
		cfg = &Config{}
	}
	report := DiagnoseReport{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Airgapped: airgapMode(),
		SarifOut:  cfg.SarifOut,
	}
	report.Checks = append(report.Checks, checkGitExecutable())
	report.Checks = append(report.Checks, checkTrunkBinary(cfg))
	report.Checks = append(report.Checks, checkAirgapEnv())
	report.Checks = append(report.Checks, checkSarifOut(cfg))
	report.Summary = summarizeDiagnoseChecks(report.Checks)
	return report
}

func summarizeDiagnoseChecks(checks []DiagnoseCheck) DiagnoseSummary {
	summary := DiagnoseSummary{Total: len(checks)}
	for _, c := range checks {
		switch c.Status {
		case diagnoseStatusOK:
			summary.OK++
		case diagnoseStatusWarn:
			summary.Warn++
		case diagnoseStatusError:
			summary.Error++
		}
	}
	return summary
}

func checkGitExecutable() DiagnoseCheck {
	path, err := exec.LookPath("git")
	if err != nil {
		return DiagnoseCheck{
			Name:           "git",
			Status:         diagnoseStatusError,
			Message:        "git executable not found in PATH",
			Recommendation: "Install git and ensure it is available to PunchTrunk.",
		}
	}
	return DiagnoseCheck{
		Name:    "git",
		Status:  diagnoseStatusOK,
		Message: fmt.Sprintf("git found at %s", path),
	}
}

func checkTrunkBinary(cfg *Config) DiagnoseCheck {
	name := "trunk_binary"
	var sources []string
	if cfg != nil && cfg.TrunkBinary != "" {
		sources = append(sources, cfg.TrunkBinary)
	}
	if env := strings.TrimSpace(os.Getenv("PUNCHTRUNK_TRUNK_BINARY")); env != "" {
		sources = append(sources, env)
	}
	sources = uniqueStrings(sources)
	for _, src := range sources {
		resolved, err := resolveTrunkBinary(src)
		if err != nil {
			return DiagnoseCheck{
				Name:           name,
				Status:         diagnoseStatusError,
				Message:        fmt.Sprintf("trunk binary %s is invalid: %v", src, err),
				Recommendation: "Provide a valid trunk executable via --trunk-binary or PUNCHTRUNK_TRUNK_BINARY.",
			}
		}
		message := fmt.Sprintf("resolved trunk executable at %s", resolved)
		cmd := exec.Command(resolved, "--version")
		cmd.Env = append(os.Environ(), "TRUNK_TELEMETRY_OPTOUT=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return DiagnoseCheck{
				Name:           name,
				Status:         diagnoseStatusWarn,
				Message:        fmt.Sprintf("%s but '--version' failed: %v (%s)", message, err, strings.TrimSpace(string(out))),
				Recommendation: "Verify the trunk binary runs without network access or rebuild the offline bundle.",
			}
		}
		version := strings.TrimSpace(string(out))
		if idx := strings.Index(version, "\n"); idx >= 0 {
			version = version[:idx]
		}
		if version == "" {
			version = "unknown version"
		}
		return DiagnoseCheck{
			Name:    name,
			Status:  diagnoseStatusOK,
			Message: fmt.Sprintf("%s (version: %s)", message, version),
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, ".trunk", "bin", trunkExecutableName())
		if resolved, err := resolveTrunkBinary(candidate); err == nil {
			return DiagnoseCheck{
				Name:           name,
				Status:         diagnoseStatusWarn,
				Message:        fmt.Sprintf("found trunk at %s but PUNCHTRUNK_TRUNK_BINARY is not set", resolved),
				Recommendation: "Export PUNCHTRUNK_TRUNK_BINARY or use --trunk-binary to avoid auto-installation attempts.",
			}
		}
	}
	return DiagnoseCheck{
		Name:           name,
		Status:         diagnoseStatusError,
		Message:        "no trunk binary detected",
		Recommendation: "Set PUNCHTRUNK_TRUNK_BINARY or pass --trunk-binary pointing at an offline bundle.",
	}
}

func checkAirgapEnv() DiagnoseCheck {
	if airgapMode() {
		return DiagnoseCheck{
			Name:    "airgap_env",
			Status:  diagnoseStatusOK,
			Message: "PUNCHTRUNK_AIRGAPPED is enabled",
		}
	}
	return DiagnoseCheck{
		Name:           "airgap_env",
		Status:         diagnoseStatusWarn,
		Message:        "PUNCHTRUNK_AIRGAPPED is not set",
		Recommendation: "Export PUNCHTRUNK_AIRGAPPED=1 to prevent PunchTrunk from downloading dependencies.",
	}
}

func checkSarifOut(cfg *Config) DiagnoseCheck {
	name := "sarif_out"
	if cfg == nil || strings.TrimSpace(cfg.SarifOut) == "" {
		return DiagnoseCheck{
			Name:           name,
			Status:         diagnoseStatusWarn,
			Message:        "sarif-out path not configured",
			Recommendation: "Use --sarif-out or --tmp-dir to direct hotspot reports to a writable location.",
		}
	}
	dir := filepath.Dir(cfg.SarifOut)
	info, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DiagnoseCheck{
				Name:           name,
				Status:         diagnoseStatusWarn,
				Message:        fmt.Sprintf("directory %s does not exist", dir),
				Recommendation: "Create the directory or point --sarif-out to an accessible path.",
			}
		}
		return DiagnoseCheck{
			Name:           name,
			Status:         diagnoseStatusError,
			Message:        fmt.Sprintf("unable to stat %s: %v", dir, err),
			Recommendation: "Verify permissions or provide --tmp-dir when using read-only workspaces.",
		}
	}
	if !info.IsDir() {
		return DiagnoseCheck{
			Name:           name,
			Status:         diagnoseStatusError,
			Message:        fmt.Sprintf("%s is not a directory", dir),
			Recommendation: "Adjust --sarif-out to target a directory path.",
		}
	}
	testFile := filepath.Join(dir, fmt.Sprintf(".punchtrunk-diagnose-%d", time.Now().UnixNano()))
	if err := os.WriteFile(testFile, []byte("diagnostic"), 0o644); err != nil {
		return DiagnoseCheck{
			Name:           name,
			Status:         diagnoseStatusError,
			Message:        fmt.Sprintf("failed to write to %s: %v", dir, err),
			Recommendation: "Adjust permissions or configure --tmp-dir to a writable location.",
		}
	}
	if err := os.Remove(testFile); err != nil {
		log.Printf("WARN: unable to clean up diagnostic file %s: %v", testFile, err)
	}
	return DiagnoseCheck{
		Name:    name,
		Status:  diagnoseStatusOK,
		Message: fmt.Sprintf("verified write access to %s", dir),
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	var result []string
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	return result
}

func resolveTrunkBinary(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("trunk binary path is empty")
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("make absolute: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory, expected executable", abs)
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("%s is not executable", abs)
	}
	return abs, nil
}

func airgapMode() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("PUNCHTRUNK_AIRGAPPED")))
	return v == "1" || v == "true" || v == "yes"
}

func ensureTrunk(ctx context.Context, cfg *Config) (string, error) {
	if path, err := exec.LookPath("trunk"); err == nil {
		if resolved, err := resolveTrunkBinary(path); err == nil {
			return resolved, nil
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, ".trunk", "bin", trunkExecutableName())
		if resolved, err := resolveTrunkBinary(candidate); err == nil {
			return resolved, nil
		}
	}
	if airgapMode() {
		if cfg != nil && cfg.Verbose {
			log.Printf("Airgapped mode enabled; skipping Trunk auto-install.")
		}
		return "", fmt.Errorf("trunk executable not found and PUNCHTRUNK_AIRGAPPED is set. Provide --trunk-binary or install trunk manually in offline environments")
	}
	if cfg != nil && cfg.Verbose {
		log.Printf("Trunk CLI not found in PATH. Attempting automatic install...")
	}
	if err := installTrunk(ctx, cfg != nil && cfg.Verbose); err != nil {
		return "", fmt.Errorf("auto-install trunk: %w", err)
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, ".trunk", "bin", trunkExecutableName())
		if resolved, err := resolveTrunkBinary(candidate); err == nil {
			return resolved, nil
		}
	}
	if path, err := exec.LookPath("trunk"); err == nil {
		if resolved, err := resolveTrunkBinary(path); err == nil {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("trunk executable not found after attempted installation")
}

func installTrunk(ctx context.Context, verbose bool) error {
	switch runtime.GOOS {
	case "windows":
		return installTrunkWindows(ctx, verbose)
	default:
		return installTrunkUnix(ctx, verbose)
	}
}

func installTrunkUnix(ctx context.Context, verbose bool) error {
	const installURL = "https://get.trunk.io"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, installURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && verbose {
			log.Printf("WARN: closing trunk installer response: %v", cerr)
		}
	}()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("download trunk installer: %s", resp.Status)
	}
	tmpFile, err := os.CreateTemp("", "trunk-install-*.sh")
	if err != nil {
		return err
	}
	defer func() {
		if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
			msg := fmt.Sprintf("WARN: removing trunk installer script %s: %v", tmpFile.Name(), removeErr)
			if verbose {
				log.Print(msg)
			} else {
				log.Printf("%s. Set TMPDIR to a writable location or clean up the file manually.", msg)
			}
		}
	}()
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		if closeErr := tmpFile.Close(); closeErr != nil && verbose {
			log.Printf("WARN: closing trunk installer temp file: %v", closeErr)
		}
		return fmt.Errorf("write installer: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpFile.Name(), 0o755); err != nil {
		return err
	}
	shell := "bash"
	if _, err := exec.LookPath(shell); err != nil {
		shell = "sh"
		if _, err := exec.LookPath(shell); err != nil {
			return fmt.Errorf("neither bash nor sh is available to run trunk installer")
		}
	}
	cmd := exec.CommandContext(ctx, shell, tmpFile.Name(), "-y")
	cmd.Env = append(os.Environ(),
		"TRUNK_INIT_NO_ANALYTICS=1",
		"TRUNK_TELEMETRY_OPTOUT=1",
	)
	var combined bytes.Buffer
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = &combined
		cmd.Stderr = &combined
	}
	if err := cmd.Run(); err != nil {
		if combined.Len() > 0 && !verbose {
			return fmt.Errorf("run trunk installer: %w: %s", err, strings.TrimSpace(combined.String()))
		}
		return fmt.Errorf("run trunk installer: %w", err)
	}
	return nil
}

func installTrunkWindows(ctx context.Context, verbose bool) error {
	script := `
$ErrorActionPreference = "Stop"
$Installer = Join-Path $env:TEMP "trunk-install-$([System.Guid]::NewGuid()).ps1"
Invoke-WebRequest -Uri "https://get.trunk.io" -UseBasicParsing -OutFile $Installer
Try {
  & $Installer -y
} Finally {
  Remove-Item $Installer -ErrorAction SilentlyContinue
}
`
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.Env = append(os.Environ(),
		"TRUNK_INIT_NO_ANALYTICS=1",
		"TRUNK_TELEMETRY_OPTOUT=1",
	)
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

func trunkExecutableName() string {
	if runtime.GOOS == "windows" {
		return "trunk.exe"
	}
	return "trunk"
}

func computeHotspots(ctx context.Context, cfg *Config) ([]Hotspot, error) {
	changed := map[string]bool{}
	if m, degraded, err := gitChangedFiles(ctx, cfg); err != nil {
		if cfg != nil && cfg.Verbose {
			log.Printf("WARN: unable to resolve changed files: %v", err)
		}
	} else {
		changed = m
		if degraded && cfg != nil && cfg.Verbose {
			log.Printf("INFO: falling back to limited git history for changed files; diff weighting may be incomplete")
		}
	}
	// Consider changed files as primary focus; also consider top churn files overall.
	churn, degradedChurn, err := gitChurn(ctx, "90 days")
	if err != nil {
		return nil, err
	}
	if degradedChurn && cfg != nil && cfg.Verbose {
		log.Printf("INFO: falling back to limited git history for churn; hotspot rankings may be partial")
	}
	// Simple complexity proxy: token density
	comp := map[string]float64{}
	for f := range churn {
		c, _ := roughComplexity(f)
		comp[f] = c
	}
	// Score and rank
	var hs []Hotspot
	// z-score complexity
	mean, std := meanStd(mapsValues(comp))
	if len(churn) == 0 && cfg != nil && cfg.Verbose {
		log.Printf("INFO: no git churn detected; hotspot report may be empty")
	}
	for f, ch := range churn {
		if _, err := os.Stat(f); err != nil {
			continue
		}
		cz := 0.0
		if std > 0 {
			cz = (comp[f] - mean) / std
		}
		score := math.Log1p(float64(ch)) * (1.0 + cz)
		// Prioritise changed files slightly
		if changed[f] {
			score *= 1.15
		}
		hs = append(hs, Hotspot{File: f, Churn: ch, Complexity: comp[f], Score: score})
	}
	sort.Slice(hs, func(i, j int) bool { return hs[i].Score > hs[j].Score })
	// Limit to reasonable number for dashboards
	if len(hs) > 500 {
		hs = hs[:500]
	}
	return hs, nil
}

func gitChangedFiles(ctx context.Context, cfg *Config) (map[string]bool, bool, error) {
	type attempt struct {
		desc string
		args []string
	}
	base := ""
	if cfg != nil {
		base = strings.TrimSpace(cfg.BaseBranch)
	}
	var attempts []attempt
	if base != "" {
		attempts = append(attempts, attempt{
			desc: fmt.Sprintf("git diff %s...HEAD", base),
			args: []string{"diff", "--name-only", base + "...HEAD"},
		})
	}
	attempts = append(attempts,
		attempt{desc: "git diff HEAD~1...HEAD", args: []string{"diff", "--name-only", "HEAD~1...HEAD"}},
		attempt{desc: "git diff HEAD^..HEAD", args: []string{"diff", "--name-only", "HEAD^..HEAD"}},
	)
	degraded := false
	var lastErr error
	var lastStderr string
	for _, att := range attempts {
		var stdout, stderr bytes.Buffer
		cmd := exec.CommandContext(ctx, "git", att.args...)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			degraded = true
			lastErr = err
			lastStderr = stderr.String()
			if cfg != nil && cfg.Verbose {
				log.Printf("DEBUG: %s failed: %v (%s)", att.desc, err, strings.TrimSpace(lastStderr))
			}
			continue
		}
		return parseNameOnly(stdout.String()), degraded, nil
	}
	if lastErr != nil {
		stderrLower := strings.ToLower(lastStderr)
		if strings.Contains(stderrLower, "bad revision") || strings.Contains(stderrLower, "unknown revision") || strings.Contains(stderrLower, "ambiguous argument") || strings.Contains(stderrLower, "no such ref") {
			return map[string]bool{}, true, nil
		}
		return map[string]bool{}, degraded, fmt.Errorf("git diff failed: %w", lastErr)
	}
	return map[string]bool{}, degraded, nil
}

func gitChurn(ctx context.Context, since string) (map[string]int, bool, error) {
	attempts := []struct {
		desc string
		args []string
	}{
		{
			desc: fmt.Sprintf("git log --since=%s --numstat", since),
			args: []string{"log", fmt.Sprintf("--since=%s", since), "--numstat", "--format=tformat:"},
		},
		{
			desc: "git log --numstat HEAD",
			args: []string{"log", "--numstat", "--format=tformat:", "HEAD"},
		},
	}
	var lastErr error
	var lastStderr string
	for idx, att := range attempts {
		churn, stderr, err := runGitNumstat(ctx, att.args...)
		if err == nil {
			return churn, idx > 0, nil
		}
		lastErr = err
		lastStderr = stderr
		if isNoHistory(stderr) {
			return map[string]int{}, true, nil
		}
	}
	if lastErr != nil {
		if isNoHistory(lastStderr) {
			return map[string]int{}, true, nil
		}
		return map[string]int{}, true, fmt.Errorf("git log failed: %w", lastErr)
	}
	return map[string]int{}, false, nil
}

func runGitNumstat(ctx context.Context, args ...string) (map[string]int, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, stderr.String(), err
	}
	return parseNumstat(stdout.String()), "", nil
}

func parseNameOnly(output string) map[string]bool {
	m := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			m[line] = true
		}
	}
	return m
}

func parseNumstat(output string) map[string]int {
	churn := map[string]int{}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 {
			added := fields[0]
			deleted := fields[1]
			file := fields[2]
			if added == "-" || deleted == "-" {
				churn[file] += 1
				continue
			}
			a := atoiSafe(added)
			d := atoiSafe(deleted)
			churn[file] += a + d
		}
	}
	return churn
}

func isNoHistory(stderr string) bool {
	s := strings.ToLower(stderr)
	return strings.Contains(s, "does not have any commits yet") ||
		strings.Contains(s, "bad revision") ||
		strings.Contains(s, "unknown revision") ||
		strings.Contains(s, "no such ref") ||
		strings.Contains(s, "shallow updates were not allowed")
}

func atoiSafe(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

func roughComplexity(path string) (float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	content := string(data)
	lines := strings.Count(content, "\n") + 1
	tokens := len(strings.Fields(content))
	if lines == 0 {
		return 0, nil
	}
	return float64(tokens) / float64(lines), nil
}

func meanStd(vals []float64) (float64, float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	mean := sum / float64(len(vals))
	var s2 float64
	for _, v := range vals {
		s2 += (v - mean) * (v - mean)
	}
	std := math.Sqrt(s2 / float64(len(vals)))
	return mean, std
}

func mapsValues(m map[string]float64) []float64 {
	out := make([]float64, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

// SARIF writing (2.1.0 minimal)
type SarifLog struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []SarifRun `json:"runs"`
}
type SarifRun struct {
	Tool    SarifTool     `json:"tool"`
	Results []SarifResult `json:"results"`
}
type SarifTool struct {
	Driver SarifDriver `json:"driver"`
}
type SarifDriver struct {
	Name           string `json:"name"`
	Version        string `json:"version,omitempty"`
	InformationURI string `json:"informationUri,omitempty"`
}
type SarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   SarifMessage    `json:"message"`
	Locations []SarifLocation `json:"locations,omitempty"`
}
type SarifMessage struct {
	Text string `json:"text"`
}
type SarifLocation struct {
	PhysicalLocation SarifPhysicalLocation `json:"physicalLocation"`
}
type SarifPhysicalLocation struct {
	ArtifactLocation SarifArtifactLocation `json:"artifactLocation"`
}
type SarifArtifactLocation struct {
	URI string `json:"uri"`
}

func writeSARIF(path string, hs []Hotspot) error {
	log := SarifLog{
		Version: "2.1.0",
		Schema:  "https://schemastore.azurewebsites.net/schemas/json/sarif-2.1.0-rtm.5.json",
		Runs: []SarifRun{{
			Tool: SarifTool{Driver: SarifDriver{
				Name:           "PunchTrunk",
				InformationURI: "https://docs.trunk.io/",
			}},
		}},
	}
	for _, h := range hs {
		msg := fmt.Sprintf("Hotspot candidate: churn=%d, complexity=%.2f, score=%.2f", h.Churn, h.Complexity, h.Score)
		log.Runs[0].Results = append(log.Runs[0].Results, SarifResult{
			RuleID:  "hotspot",
			Level:   "note",
			Message: SarifMessage{Text: msg},
			Locations: []SarifLocation{{
				PhysicalLocation: SarifPhysicalLocation{
					ArtifactLocation: SarifArtifactLocation{URI: filepath.ToSlash(h.File)},
				},
			}},
		})
	}
	tmp := &bytes.Buffer{}
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(&log); err != nil {
		return err
	}
	if err := os.WriteFile(path, tmp.Bytes(), 0o644); err != nil {
		return err
	}
	return nil
}
