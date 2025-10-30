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

	yaml "gopkg.in/yaml.v3"
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

type LogFields map[string]any

type eventLogger struct {
	mu    sync.Mutex
	json  bool
	std   *log.Logger
	write io.Writer
}

func newEventLogger(w io.Writer, jsonMode bool) *eventLogger {
	if w == nil {
		w = os.Stderr
	}
	return &eventLogger{
		json:  jsonMode,
		std:   log.New(w, "", log.LstdFlags),
		write: w,
	}
}

func (l *eventLogger) emit(level, message string, fields LogFields) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.json {
		payload := make(map[string]any, len(fields)+3)
		payload["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
		payload["level"] = level
		payload["message"] = message
		for k, v := range fields {
			payload[k] = v
		}
		data, err := json.Marshal(payload)
		if err != nil {
			l.std.Printf("ERROR: json log marshal failed: %v", err)
			l.std.Printf("ERROR: original message: %s", message)
			return
		}
		if _, err := l.write.Write(append(data, '\n')); err != nil {
			l.std.Printf("ERROR: json log write failed: %v", err)
		}
		return
	}
	text := message
	if len(fields) > 0 {
		var extras []string
		for k, v := range fields {
			if k == "event" {
				if s, ok := v.(string); ok {
					text = s
					continue
				}
			}
			extras = append(extras, fmt.Sprintf("%s=%v", k, v))
		}
		sort.Strings(extras)
		if len(extras) > 0 {
			text = fmt.Sprintf("%s | %s", text, strings.Join(extras, " "))
		}
	}
	switch level {
	case "warn":
		l.std.Printf("WARN: %s", text)
	case "error":
		l.std.Printf("ERROR: %s", text)
	default:
		l.std.Printf("INFO: %s", text)
	}
}

func (l *eventLogger) Infof(format string, args ...any) {
	l.emit("info", fmt.Sprintf(format, args...), nil)
}

func (l *eventLogger) Warnf(format string, args ...any) {
	l.emit("warn", fmt.Sprintf(format, args...), nil)
}

func (l *eventLogger) Errorf(format string, args ...any) {
	l.emit("error", fmt.Sprintf(format, args...), nil)
}

func (l *eventLogger) Fatalf(format string, args ...any) {
	l.emit("error", fmt.Sprintf(format, args...), nil)
	os.Exit(1)
}

func (l *eventLogger) Event(level, event string, fields LogFields) {
	if fields == nil {
		fields = LogFields{}
	}
	copyFields := make(LogFields, len(fields)+1)
	for k, v := range fields {
		copyFields[k] = v
	}
	copyFields["event"] = event
	l.emit(level, event, copyFields)
}

var defaultLogger = newEventLogger(os.Stderr, false)

type Config struct {
	Modes              []string
	Autofix            string
	BaseBranch         string
	MaxProcs           int
	Timeout            time.Duration
	SarifOut           string
	Verbose            bool
	JSONLogs           bool
	DryRun             bool
	TmpDir             string
	ShowVersion        bool
	TrunkPath          string
	TrunkConfigDir     string
	TrunkArgs          []string
	TrunkBinary        string
	TrunkVersion       string
	TrunkCacheDir      string
	TrunkManifest      *bundleManifest
	TrunkConfig        *trunkYAML
	ManifestPath       string
	ToolHealthFormat   string
	ToolHealthJSONPath string
	logger             *eventLogger
	tmpDirResolved     string
	tmpDirErr          error
	tmpDirOnce         sync.Once
}

type trunkYAML struct {
	CLI struct {
		Version string `yaml:"version"`
	} `yaml:"cli"`
	Plugins struct {
		Sources []trunkPluginSource `yaml:"sources"`
	} `yaml:"plugins"`
	Runtimes struct {
		Enabled []string `yaml:"enabled"`
	} `yaml:"runtimes"`
	Lint struct {
		Enabled []string `yaml:"enabled"`
	} `yaml:"lint"`
}

type trunkPluginSource struct {
	ID  string `yaml:"id" json:"id"`
	Ref string `yaml:"ref" json:"ref"`
	URI string `yaml:"uri" json:"uri"`
}

type bundleManifest struct {
	CreatedAt          string   `json:"created_at"`
	PunchTrunkBinary   string   `json:"punchtrunk_binary"`
	TrunkBinary        string   `json:"trunk_binary"`
	TrunkVersion       string   `json:"trunk_version"`
	CacheIncluded      bool     `json:"cache_included"`
	ConfigRelativePath string   `json:"config_relative_path"`
	CacheRelativePath  string   `json:"cache_relative_path"`
	TrunkCLIVersion    string   `json:"trunk_cli_version,omitempty"`
	TrunkConfigSHA256  string   `json:"trunk_config_sha256,omitempty"`
	HydrateAttempted   bool     `json:"hydrate_attempted,omitempty"`
	HydrateStatus      string   `json:"hydrate_status,omitempty"`
	HydrateWarnings    []string `json:"hydrate_warnings,omitempty"`
	CacheDirSource     string   `json:"cache_dir_source,omitempty"`
}

type toolHealthReport struct {
	Timestamp     string            `json:"timestamp"`
	ConfigDir     string            `json:"config_dir,omitempty"`
	CacheDir      string            `json:"cache_dir,omitempty"`
	ManifestPath  string            `json:"manifest_path,omitempty"`
	Manifest      *bundleManifest   `json:"manifest,omitempty"`
	Trunk         toolHealthVersion `json:"trunk"`
	PluginSources []toolHealthItem  `json:"plugin_sources,omitempty"`
	Runtimes      []toolHealthItem  `json:"runtimes,omitempty"`
	Linters       []toolHealthItem  `json:"linters,omitempty"`
	Warnings      []string          `json:"warnings,omitempty"`
}

type toolHealthVersion struct {
	Expected string `json:"expected,omitempty"`
	Detected string `json:"detected,omitempty"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
}

type toolHealthItem struct {
	Name      string `json:"name"`
	CachePath string `json:"cache_path,omitempty"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

func main() {
	cfg := parseFlags()
	cfg.log()

	if cfg.ShowVersion {
		fmt.Printf("PunchTrunk version %s\n", Version)
		return
	}

	if cfg.DryRun {
		if err := executeDryRun(cfg); err != nil {
			cfg.log().Fatalf("dry-run failed: %v", err)
		}
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
			cfg.log().Fatalf("environment setup failed: %v", err)
		}
		cfg.log().Event("info", "environment.ready", LogFields{
			"trunk_path": cfg.TrunkPath,
		})
	}

	for idx, raw := range cfg.Modes {
		mode := strings.TrimSpace(strings.ToLower(raw))
		if mode == "" {
			continue
		}
		var err error
		cfg.log().Event("info", "mode.start", LogFields{
			"mode":         mode,
			"mode_index":   idx,
			"trunk_path":   cfg.trunkBinary(),
			"sarif_out":    cfg.SarifOut,
			"autofix_mode": cfg.Autofix,
		})
		modeStart := time.Now()
		switch mode {
		case "fmt":
			err = runTrunkFmt(ctx, cfg)
		case "lint":
			err = runTrunkCheck(ctx, cfg)
		case "hotspots":
			err = runHotspots(ctx, cfg)
		case "diagnose-airgap":
			err = runDiagnoseAirgap(cfg)
		case "tool-health":
			err = runToolHealth(ctx, cfg)
		default:
			if cfg.Verbose {
				cfg.log().Warnf("Skipping unknown mode %q", raw)
			}
			continue
		}
		if err != nil {
			duration := time.Since(modeStart)
			cfg.log().Event("error", "mode.error", LogFields{
				"mode":        mode,
				"mode_index":  idx,
				"duration_ms": duration.Milliseconds(),
				"error":       err.Error(),
			})
			if mode == "hotspots" {
				cfg.log().Warnf("%s failed: %v", mode, err)
				continue
			}
			cfg.log().Fatalf("%s failed: %v", mode, err)
		}
		duration := time.Since(modeStart)
		cfg.log().Event("info", "mode.finish", LogFields{
			"mode":        mode,
			"mode_index":  idx,
			"duration_ms": duration.Milliseconds(),
		})
	}

	if exitErr != nil {
		os.Exit(1)
	}
}

func (cfg *Config) log() *eventLogger {
	if cfg == nil {
		return defaultLogger
	}
	if cfg.logger == nil {
		cfg.logger = newEventLogger(os.Stderr, cfg.JSONLogs)
	}
	return cfg.logger
}

func (cfg *Config) resolveTmpDir() (string, error) {
	if cfg == nil {
		dir := os.TempDir()
		if dir == "" {
			return "", fmt.Errorf("system temp dir unavailable")
		}
		return dir, nil
	}
	cfg.tmpDirOnce.Do(func() {
		base := strings.TrimSpace(cfg.TmpDir)
		if base == "" {
			base = os.TempDir()
		} else {
			if !filepath.IsAbs(base) {
				cwd, err := os.Getwd()
				if err != nil {
					cfg.tmpDirErr = fmt.Errorf("resolve tmp-dir: %w", err)
					return
				}
				base = filepath.Join(cwd, base)
			}
			base = filepath.Clean(base)
		}
		if base == "" {
			cfg.tmpDirErr = fmt.Errorf("tmp-dir resolved to empty path")
			return
		}
		if err := os.MkdirAll(base, 0o755); err != nil {
			cfg.tmpDirErr = fmt.Errorf("ensure tmp-dir %s: %w", base, err)
			return
		}
		cfg.tmpDirResolved = base
	})
	if cfg.tmpDirErr != nil {
		return "", cfg.tmpDirErr
	}
	if cfg.tmpDirResolved == "" {
		dir := os.TempDir()
		if dir == "" {
			return "", fmt.Errorf("system temp dir unavailable")
		}
		return dir, nil
	}
	return cfg.tmpDirResolved, nil
}

func (cfg *Config) tempDir() string {
	dir, err := cfg.resolveTmpDir()
	if err != nil {
		cfg.log().Warnf("tmp-dir resolution failed: %v", err)
		fallback := os.TempDir()
		if fallback == "" {
			return "."
		}
		return fallback
	}
	return dir
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
	var jsonLogs bool
	var dryRun bool
	var tmpDir string
	var autofix string
	var version bool
	var trunkConfigDir string
	var trunkBinary string
	var trunkArgs multiFlag
	var toolHealthFormat string
	var toolHealthJSON string
	flag.StringVar(&modes, "mode", "fmt,lint,hotspots", "Comma-separated phases: fmt,lint,hotspots")
	flag.StringVar(&autofix, "autofix", "fmt", "Autofix scope: none|fmt|lint|all")
	flag.StringVar(&base, "base-branch", "origin/main", "Base branch for change detection")
	flag.IntVar(&maxProcs, "max-procs", 0, "Parallelism cap (0 = CPU cores)")
	flag.IntVar(&timeoutSec, "timeout", 900, "Overall timeout in seconds (0 to disable)")
	flag.StringVar(&sarifOut, "sarif-out", "reports/hotspots.sarif", "SARIF output path for hotspots")
	flag.BoolVar(&verbose, "verbose", false, "Verbose logs")
	flag.BoolVar(&jsonLogs, "json-logs", false, "Emit structured JSON logs")
	flag.BoolVar(&dryRun, "dry-run", false, "Preview planned commands without executing them")
	flag.StringVar(&tmpDir, "tmp-dir", "", "Override temporary directory PunchTrunk uses for fallbacks and installers")
	flag.BoolVar(&version, "version", false, "Show version and exit")
	flag.StringVar(&trunkConfigDir, "trunk-config-dir", "", "Override Trunk config directory (defaults to repo autodetect)")
	flag.StringVar(&trunkBinary, "trunk-binary", "", "Explicit path to trunk executable (for airgapped runners)")
	flag.Var(&trunkArgs, "trunk-arg", "Additional argument to pass to trunk CLI (repeatable)")
	flag.StringVar(&toolHealthFormat, "tool-health-format", "json", "Output format for tool-health: json|summary")
	flag.StringVar(&toolHealthJSON, "tool-health-json", "", "Optional file path to write tool-health JSON report")
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

	if !jsonLogs {
		if env := strings.TrimSpace(os.Getenv("PUNCHTRUNK_JSON_LOGS")); env != "" {
			if parsed, err := strconv.ParseBool(env); err == nil {
				jsonLogs = parsed
			}
		}
	}

	if tmpDir == "" {
		if env := strings.TrimSpace(os.Getenv("PUNCHTRUNK_TMP_DIR")); env != "" {
			tmpDir = env
		}
	}

	return &Config{
		Modes:              modeList,
		Autofix:            strings.ToLower(strings.TrimSpace(autofix)),
		BaseBranch:         base,
		MaxProcs:           maxProcs,
		Timeout:            timeout,
		SarifOut:           filepath.Clean(sarifOut),
		Verbose:            verbose,
		JSONLogs:           jsonLogs,
		DryRun:             dryRun,
		TmpDir:             strings.TrimSpace(tmpDir),
		ShowVersion:        version,
		TrunkConfigDir:     trunkConfigDir,
		TrunkArgs:          trunkArgs,
		TrunkBinary:        trunkBinary,
		ToolHealthFormat:   strings.TrimSpace(toolHealthFormat),
		ToolHealthJSONPath: strings.TrimSpace(toolHealthJSON),
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

func trunkFmtArgs(cfg *Config) []string {
	args := []string{"fmt"}
	if cfg != nil {
		args = append(args, cfg.TrunkArgs...)
	}
	return args
}

func trunkCheckArgs(cfg *Config) []string {
	args := []string{"check"}
	scope := ""
	if cfg != nil {
		scope = cfg.Autofix
	}
	switch scope {
	case "all":
		args = append(args, "--fix")
	case "lint":
		args = append(args, "--fix")
	case "fmt":
		// default path: no extra flag
	case "none":
		// no autofix
	default:
		// treat unknown as none
	}
	if cfg != nil {
		args = append(args, cfg.TrunkArgs...)
	}
	return args
}

func runTrunkFmt(ctx context.Context, cfg *Config) error {
	args := trunkFmtArgs(cfg)
	cmd := exec.CommandContext(ctx, cfg.trunkBinary(), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	applyTrunkCommandEnv(cmd, cfg)
	maybeWarnCompetingTools("fmt", cfg)
	if cfg.Verbose {
		cfg.log().Infof("Running: %s %s", cfg.trunkBinary(), strings.Join(args, " "))
	}
	return cmd.Run()
}

func runTrunkCheck(ctx context.Context, cfg *Config) error {
	args := trunkCheckArgs(cfg)
	// Let trunk decide changed files via hold-the-line; base branch is read from trunk.yaml.
	cmd := exec.CommandContext(ctx, cfg.trunkBinary(), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	applyTrunkCommandEnv(cmd, cfg)
	maybeWarnCompetingTools("lint", cfg)
	if cfg.Verbose {
		cfg.log().Infof("Running: %s %s", cfg.trunkBinary(), strings.Join(args, " "))
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
			cfg.log().Warnf("Hotspots computed (%d results) but SARIF output path is empty", len(hs))
		}
		return nil
	}
	dir := filepath.Dir(cfg.SarifOut)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fallback, ok := sarifFallbackPath(cfg, cfg.SarifOut, err)
			if !ok {
				return fmt.Errorf("create SARIF directory %s: %w", dir, err)
			}
			cfg.log().Warnf("unable to create SARIF directory %s: %v; writing to %s instead", dir, err, fallback)
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
	cfg.log().Event("info", "sarif.write", LogFields{
		"sarif_out": cfg.SarifOut,
		"count":     len(hs),
	})
	return nil
}

func sarifFallbackPath(cfg *Config, current string, mkdirErr error) (string, bool) {
	if current == "" {
		return "", false
	}
	if !isPermissionOrReadOnly(mkdirErr) {
		return "", false
	}
	base := os.TempDir()
	if cfg != nil {
		if dir, err := cfg.resolveTmpDir(); err == nil {
			base = dir
		}
	}
	fallbackDir := filepath.Join(base, "punchtrunk", "reports")
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
	if cfg != nil {
		if cfg.TrunkConfigDir != "" {
			env = appendEnvIfMissing(env, "TRUNK_CONFIG_DIR", cfg.TrunkConfigDir)
		}
		if cfg.TrunkCacheDir != "" {
			env = appendEnvIfMissing(env, "TRUNK_CACHE_DIR", cfg.TrunkCacheDir)
		}
	}
	cmd.Env = env
}

func maybeWarnCompetingTools(mode string, cfg *Config) {
	conflicts := detectCompetingToolConfigs(mode)
	if len(conflicts) == 0 {
		return
	}
	logger := defaultLogger
	if cfg != nil {
		logger = cfg.log()
	}
	for _, msg := range conflicts {
		conflictMu.Lock()
		if _, ok := seenConflictMessages[msg]; !ok {
			logger.Infof("%s", msg)
			seenConflictMessages[msg] = struct{}{}
			conflictGuidanceOnce.Do(func() {
				logger.Infof("Use --trunk-config-dir to point at the desired Trunk config or repeat --trunk-arg to forward filters that avoid tool overlap.")
			})
		}
		conflictMu.Unlock()
	}
}

func executeDryRun(cfg *Config) error {
	plan, err := buildDryRunPlan(cfg)
	if err != nil {
		return err
	}
	cfg.log().Event("info", "dryrun.plan", LogFields{
		"mode_count":   len(plan.Modes),
		"trunk_status": plan.Trunk.Status,
		"auto_install": plan.Trunk.AutoInstall,
	})
	plan.Print(os.Stdout)
	return nil
}

type dryRunPlan struct {
	Trunk     dryRunTrunk
	Env       []string
	TrunkArgs []string
	SarifOut  string
	Modes     []dryRunMode
	Warnings  []string
	Notes     []string
}

type dryRunTrunk struct {
	Path        string
	Source      string
	Status      string
	AutoInstall bool
	Airgapped   bool
	Version     string
}

type dryRunMode struct {
	Name        string
	Command     []string
	Description string
}

func buildDryRunPlan(cfg *Config) (*dryRunPlan, error) {
	if cfg == nil {
		cfg = &Config{}
	}
	plan := &dryRunPlan{
		SarifOut:  cfg.SarifOut,
		TrunkArgs: append([]string(nil), cfg.TrunkArgs...),
	}
	if cfg.TrunkConfigDir != "" {
		plan.Env = append(plan.Env, fmt.Sprintf("TRUNK_CONFIG_DIR=%s", cfg.TrunkConfigDir))
	}
	if cfg.TrunkCacheDir != "" {
		plan.Env = append(plan.Env, fmt.Sprintf("TRUNK_CACHE_DIR=%s", cfg.TrunkCacheDir))
	}
	trunkInfo, warnings := resolveDryRunTrunk(cfg)
	trunkInfo.Version = cfg.TrunkVersion
	plan.Trunk = trunkInfo
	plan.Warnings = append(plan.Warnings, warnings...)
	modes := cfg.Modes
	if len(modes) == 0 {
		modes = []string{"fmt", "lint", "hotspots"}
	}
	for _, raw := range modes {
		mode := strings.TrimSpace(strings.ToLower(raw))
		if mode == "" {
			continue
		}
		modePlan := dryRunMode{Name: mode}
		switch mode {
		case "fmt":
			args := trunkFmtArgs(cfg)
			modePlan.Command = prependCommand(plan.Trunk.displayCommand(), args)
			modePlan.Description = "format code via trunk fmt"
		case "lint":
			args := trunkCheckArgs(cfg)
			modePlan.Command = prependCommand(plan.Trunk.displayCommand(), args)
			modePlan.Description = "run trunk lint checks"
		case "hotspots":
			if strings.TrimSpace(cfg.SarifOut) != "" {
				modePlan.Description = fmt.Sprintf("compute hotspots and write SARIF to %s", cfg.SarifOut)
			} else {
				modePlan.Description = "compute hotspots (no SARIF destination configured)"
			}
		case "diagnose-airgap":
			modePlan.Command = []string{"punchtrunk", "--mode", "diagnose-airgap"}
			modePlan.Description = "emit JSON diagnostics about offline readiness"
		case "tool-health":
			modePlan.Command = []string{"punchtrunk", "--mode", "tool-health"}
			modePlan.Description = "emit cache hydration and version status"
		default:
			modePlan.Description = "mode not recognized; it would be skipped"
		}
		plan.Modes = append(plan.Modes, modePlan)
	}
	if len(plan.Modes) == 0 {
		plan.Notes = append(plan.Notes, "No modes were selected; nothing would run.")
	}
	plan.Notes = append(plan.Notes, "No commands executed because --dry-run is enabled.")
	return plan, nil
}

func resolveDryRunTrunk(cfg *Config) (dryRunTrunk, []string) {
	info := dryRunTrunk{
		Status:    "missing",
		Airgapped: airgapMode(),
	}
	var warnings []string
	seen := map[string]struct{}{}
	try := func(raw, source string) bool {
		if raw == "" {
			return false
		}
		if _, ok := seen[raw]; ok {
			return false
		}
		seen[raw] = struct{}{}
		resolved, err := resolveTrunkBinary(raw)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s %s is invalid: %v", source, raw, err))
			return false
		}
		info.Path = resolved
		info.Source = source
		info.Status = "available"
		return true
	}
	if cfg != nil && try(cfg.TrunkBinary, "--trunk-binary") {
		return info, warnings
	}
	if env := strings.TrimSpace(os.Getenv("PUNCHTRUNK_TRUNK_BINARY")); try(env, "PUNCHTRUNK_TRUNK_BINARY") {
		return info, warnings
	}
	if path, err := exec.LookPath(trunkExecutableName()); err == nil {
		if try(path, "PATH") {
			return info, warnings
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, ".trunk", "bin", trunkExecutableName())
		if try(candidate, "~/.trunk/bin") {
			return info, warnings
		}
	}
	if !info.Airgapped {
		info.AutoInstall = true
	}
	return info, warnings
}

func (p *dryRunPlan) Print(w io.Writer) {
	if p == nil {
		return
	}
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintln(w, "Dry run summary (no commands executed)")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Trunk binary: %s\n", p.Trunk.summary())
	if len(p.Env) > 0 {
		fmt.Fprintln(w, "Environment exports:")
		for _, kv := range p.Env {
			fmt.Fprintf(w, "  %s\n", kv)
		}
	}
	if len(p.TrunkArgs) > 0 {
		fmt.Fprintf(w, "Additional trunk arguments: %s\n", strings.Join(p.TrunkArgs, ", "))
	}
	if strings.TrimSpace(p.SarifOut) != "" {
		fmt.Fprintf(w, "SARIF output path: %s\n", p.SarifOut)
	}
	if len(p.Modes) > 0 {
		fmt.Fprintln(w, "Planned modes:")
		for idx, mode := range p.Modes {
			line := fmt.Sprintf("  %d. %s", idx+1, mode.Name)
			if len(mode.Command) > 0 {
				line = fmt.Sprintf("%s -> %s", line, strings.Join(mode.Command, " "))
			}
			fmt.Fprintln(w, line)
			if mode.Description != "" {
				fmt.Fprintf(w, "     %s\n", mode.Description)
			}
		}
	} else {
		fmt.Fprintln(w, "Planned modes: none")
	}
	if len(p.Warnings) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Warnings:")
		for _, warn := range p.Warnings {
			fmt.Fprintf(w, "  - %s\n", warn)
		}
	}
	if len(p.Notes) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Notes:")
		for _, note := range p.Notes {
			fmt.Fprintf(w, "  - %s\n", note)
		}
	}
}

func (t dryRunTrunk) summary() string {
	if t.Status == "available" {
		path := t.Path
		if path == "" {
			path = trunkExecutableName()
		}
		if t.Source != "" {
			if strings.TrimSpace(t.Version) != "" {
				return fmt.Sprintf("%s (source: %s, version: %s)", path, t.Source, t.Version)
			}
			return fmt.Sprintf("%s (source: %s)", path, t.Source)
		}
		if strings.TrimSpace(t.Version) != "" {
			return fmt.Sprintf("%s (version: %s)", path, t.Version)
		}
		return path
	}
	if t.AutoInstall {
		return "not detected; PunchTrunk would attempt to auto-install trunk"
	}
	if t.Airgapped {
		return "not detected; provide --trunk-binary or PUNCHTRUNK_TRUNK_BINARY when running offline"
	}
	return "not detected"
}

func (t dryRunTrunk) displayCommand() string {
	if t.Path != "" {
		return t.Path
	}
	return trunkExecutableName()
}

func prependCommand(command string, args []string) []string {
	out := make([]string, 0, 1+len(args))
	if command == "" {
		command = trunkExecutableName()
	}
	out = append(out, command)
	out = append(out, args...)
	return out
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

func detectTrunkConfigDir(start string) (string, error) {
	var err error
	if strings.TrimSpace(start) == "" {
		start, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("getwd: %w", err)
		}
	}
	start = filepath.Clean(start)
	if start == "" || start == string(filepath.Separator) {
		return "", nil
	}
	prev := ""
	dir := start
	for {
		candidate := filepath.Join(dir, ".trunk", "trunk.yaml")
		info, statErr := os.Stat(candidate)
		if statErr == nil && !info.IsDir() {
			return filepath.Dir(candidate), nil
		}
		if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", fmt.Errorf("stat %s: %w", candidate, statErr)
		}
		if dir == prev {
			break
		}
		prev = dir
		dir = filepath.Dir(dir)
		if dir == "" {
			break
		}
	}
	return "", nil
}

func loadTrunkConfig(dir string) (*trunkYAML, error) {
	if dir == "" {
		return nil, fmt.Errorf("trunk config directory is empty")
	}
	file := filepath.Join(dir, "trunk.yaml")
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", file, err)
	}
	var cfg trunkYAML
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}
	return &cfg, nil
}

func detectBundleManifest(cfg *Config) (*bundleManifest, string, error) {
	var candidates []string
	if home := strings.TrimSpace(os.Getenv("PUNCHTRUNK_HOME")); home != "" {
		candidates = append(candidates, filepath.Join(home, "manifest.json"))
	}
	if cfg != nil && strings.TrimSpace(cfg.TrunkConfigDir) != "" {
		t := filepath.Clean(cfg.TrunkConfigDir)
		candidates = append(candidates,
			filepath.Join(filepath.Dir(t), "manifest.json"),
			filepath.Join(filepath.Dir(filepath.Dir(t)), "manifest.json"),
		)
	}
	seen := map[string]struct{}{}
	for _, raw := range candidates {
		if raw == "" {
			continue
		}
		abs, err := filepath.Abs(raw)
		if err != nil {
			continue
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		data, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		var manifest bundleManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			continue
		}
		return &manifest, abs, nil
	}
	return nil, "", nil
}

func detectTrunkCacheDir(cfg *Config) string {
	if env := strings.TrimSpace(os.Getenv("TRUNK_CACHE_DIR")); env != "" {
		return filepath.Clean(env)
	}
	if cfg != nil && cfg.TrunkManifest != nil {
		if base := manifestBaseDir(cfg.ManifestPath); base != "" {
			rel := strings.TrimSpace(cfg.TrunkManifest.CacheRelativePath)
			if rel != "" {
				candidate := filepath.Join(base, rel)
				if pathExists(candidate) {
					return candidate
				}
			}
		}
	}
	if home := strings.TrimSpace(os.Getenv("PUNCHTRUNK_HOME")); home != "" {
		candidate := filepath.Join(home, "trunk", "cache")
		if pathExists(candidate) {
			return candidate
		}
	}
	if cfg != nil && strings.TrimSpace(cfg.TrunkConfigDir) != "" {
		candidate := filepath.Join(filepath.Dir(cfg.TrunkConfigDir), "cache")
		if pathExists(candidate) {
			return candidate
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidate := filepath.Join(home, ".cache", "trunk")
		return candidate
	}
	return ""
}

func manifestBaseDir(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return ""
	}
	return filepath.Dir(abs)
}

func pathExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func appendEnvIfMissing(env []string, key, value string) []string {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return env
	}
	prefix := key + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return env
		}
	}
	return append(env, fmt.Sprintf("%s=%s", key, value))
}

func cachePath(base string, parts ...string) string {
	if strings.TrimSpace(base) == "" {
		return ""
	}
	segments := append([]string{base}, parts...)
	return filepath.Join(segments...)
}

func detectTrunkVersion(ctx context.Context, trunkPath string) (string, error) {
	if strings.TrimSpace(trunkPath) == "" {
		return "", fmt.Errorf("trunk path is empty")
	}
	cmd := exec.CommandContext(ctx, trunkPath, "--version")
	cmd.Env = append(os.Environ(), "TRUNK_TELEMETRY_OPTOUT=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("trunk --version: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	version := strings.TrimSpace(string(out))
	if idx := strings.Index(version, "\n"); idx >= 0 {
		version = strings.TrimSpace(version[:idx])
	}
	return version, nil
}

func normalizeTrunkVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "trunk version ")
	v = strings.TrimPrefix(v, "trunk ")
	return strings.TrimSpace(v)
}

func trunkVersionMatches(expected, actual string) bool {
	if strings.TrimSpace(expected) == "" || strings.TrimSpace(actual) == "" {
		return true
	}
	actualNorm := normalizeTrunkVersion(actual)
	if actualNorm == expected {
		return true
	}
	if strings.Contains(actual, expected) {
		return true
	}
	for _, field := range strings.Fields(actualNorm) {
		if field == expected {
			return true
		}
	}
	return false
}

func splitToolReference(ref string) (string, string) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", ""
	}
	parts := strings.SplitN(ref, "@", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return strings.TrimSpace(ref), ""
}

func ensureEnvironment(ctx context.Context, cfg *Config) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is required: %w", err)
	}

	if _, err := cfg.resolveTmpDir(); err != nil {
		return err
	}

	if cfg.TrunkConfigDir == "" {
		if detected, err := detectTrunkConfigDir(""); err != nil {
			if cfg.Verbose {
				cfg.log().Warnf("trunk config discovery failed: %v", err)
			}
		} else if detected != "" {
			cfg.TrunkConfigDir = detected
			if cfg.Verbose {
				cfg.log().Infof("Detected Trunk config at %s", cfg.TrunkConfigDir)
			}
		}
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
		configPath := filepath.Join(abs, "trunk.yaml")
		if _, err := os.Stat(configPath); errors.Is(err, os.ErrNotExist) {
			if cfg.Verbose {
				cfg.log().Warnf("trunk-config-dir %s does not contain trunk.yaml; trunk will rely on discovery", abs)
			}
		} else if err != nil {
			return fmt.Errorf("trunk-config-dir %s: %w", abs, err)
		} else {
			if parsed, err := loadTrunkConfig(abs); err != nil {
				cfg.log().Warnf("failed to parse %s: %v", configPath, err)
			} else {
				cfg.TrunkConfig = parsed
			}
		}
	}

	if cfg.TrunkConfig == nil {
		if detected, err := detectTrunkConfigDir(cfg.TrunkConfigDir); err == nil && detected != "" {
			if parsed, perr := loadTrunkConfig(detected); perr == nil {
				cfg.TrunkConfig = parsed
				if cfg.TrunkConfigDir == "" {
					cfg.TrunkConfigDir = detected
				}
			}
		}
	}

	manifest, manifestPath, manifestErr := detectBundleManifest(cfg)
	if manifestErr != nil {
		if cfg.Verbose {
			cfg.log().Warnf("manifest detection failed: %v", manifestErr)
		}
	} else {
		cfg.TrunkManifest = manifest
		cfg.ManifestPath = manifestPath
	}

	cfg.TrunkCacheDir = detectTrunkCacheDir(cfg)
	if cfg.TrunkCacheDir != "" {
		if err := os.MkdirAll(cfg.TrunkCacheDir, 0o755); err != nil && cfg.Verbose {
			cfg.log().Warnf("unable to ensure cache directory %s: %v", cfg.TrunkCacheDir, err)
		}
	}

	if cfg.TrunkBinary != "" {
		resolved, err := resolveTrunkBinary(cfg.TrunkBinary)
		if err != nil {
			return fmt.Errorf("trunk-binary validation: %w", err)
		}
		cfg.TrunkPath = resolved
		if version, err := detectTrunkVersion(ctx, cfg.TrunkPath); err == nil {
			cfg.TrunkVersion = version
			if cfg.TrunkConfig != nil && cfg.TrunkConfig.CLI.Version != "" && !trunkVersionMatches(cfg.TrunkConfig.CLI.Version, version) {
				cfg.log().Warnf("Trunk CLI version mismatch: config expects %s but resolved %s", cfg.TrunkConfig.CLI.Version, version)
			}
		} else if cfg.Verbose {
			cfg.log().Warnf("unable to resolve trunk version: %v", err)
		}
		if cfg.Verbose {
			cfg.log().Infof("Using user-supplied trunk binary: %s", resolved)
		}
		return nil
	}

	trunkPath, err := ensureTrunk(ctx, cfg)
	if err != nil {
		return err
	}
	cfg.TrunkPath = trunkPath
	if version, err := detectTrunkVersion(ctx, cfg.TrunkPath); err == nil {
		cfg.TrunkVersion = version
		if cfg.TrunkConfig != nil && cfg.TrunkConfig.CLI.Version != "" && !trunkVersionMatches(cfg.TrunkConfig.CLI.Version, version) {
			cfg.log().Warnf("Trunk CLI version mismatch: config expects %s but resolved %s", cfg.TrunkConfig.CLI.Version, version)
		}
	} else if cfg.Verbose {
		cfg.log().Warnf("unable to resolve trunk version: %v", err)
	}
	return nil
}

func runDiagnoseAirgap(cfg *Config) error {
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

func runToolHealth(ctx context.Context, cfg *Config) error {
	if cfg == nil {
		cfg = &Config{}
	}
	report := toolHealthReport{
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
		ConfigDir:    cfg.TrunkConfigDir,
		CacheDir:     cfg.TrunkCacheDir,
		ManifestPath: cfg.ManifestPath,
		Manifest:     cfg.TrunkManifest,
	}
	expectedVersion := ""
	if cfg.TrunkConfig != nil {
		report.Trunk.Expected = strings.TrimSpace(cfg.TrunkConfig.CLI.Version)
		expectedVersion = report.Trunk.Expected
	}
	report.Trunk.Detected = strings.TrimSpace(cfg.TrunkVersion)
	switch {
	case report.Trunk.Detected == "":
		report.Trunk.Status = "unknown"
		report.Trunk.Message = "trunk version not resolved"
	case expectedVersion == "":
		report.Trunk.Status = "detected"
		report.Trunk.Message = "no CLI version pinned in trunk.yaml"
	case trunkVersionMatches(expectedVersion, report.Trunk.Detected):
		report.Trunk.Status = "match"
	default:
		report.Trunk.Status = "mismatch"
		report.Trunk.Message = fmt.Sprintf("expected %s but detected %s", expectedVersion, report.Trunk.Detected)
	}

	cacheDir := strings.TrimSpace(cfg.TrunkCacheDir)
	cacheAvailable := cacheDir != "" && pathExists(cacheDir)
	var warnings []string
	issues := false
	if cacheDir == "" {
		warnings = append(warnings, "TRUNK_CACHE_DIR not resolved; cache hydration status is unknown")
	} else if !cacheAvailable {
		warnings = append(warnings, fmt.Sprintf("cache directory %s does not exist", cacheDir))
	}

	if cfg.TrunkManifest != nil && !cfg.TrunkManifest.CacheIncluded {
		warnings = append(warnings, "bundle manifest indicates cache was not included during build")
	}

	buildItem := func(name, pathWhenKnown string, hydrated bool, message string) toolHealthItem {
		status := "hydrated"
		if !hydrated {
			status = "missing"
			if message == "" {
				message = "cache entry not found"
			}
		}
		if cacheDir == "" {
			status = "unknown"
			if message == "" {
				message = "cache directory not resolved"
			}
		}
		if cacheDir != "" && !cacheAvailable {
			status = "missing"
			if message == "" {
				message = "cache directory missing"
			}
		}
		return toolHealthItem{Name: name, CachePath: pathWhenKnown, Status: status, Message: message}
	}

	if cfg.TrunkConfig != nil {
		for _, src := range cfg.TrunkConfig.Plugins.Sources {
			name := strings.TrimSpace(src.ID)
			if src.Ref != "" {
				name = fmt.Sprintf("%s@%s", strings.TrimSpace(src.ID), strings.TrimSpace(src.Ref))
			}
			cacheEntry := ""
			hydrated := false
			message := ""
			if cacheDir == "" {
				message = "cache directory not resolved"
			} else if src.ID == "" || src.Ref == "" {
				message = "plugin source missing id or ref"
				statusItem := buildItem(name, cacheEntry, false, message)
				report.PluginSources = append(report.PluginSources, statusItem)
				continue
			} else {
				cacheEntry = cachePath(cacheDir, "plugins", strings.TrimSpace(src.ID), strings.TrimSpace(src.Ref))
				hydrated = pathExists(cacheEntry)
				if !hydrated {
					message = "plugin cache not found"
					warnings = append(warnings, fmt.Sprintf("missing plugin cache for %s (%s)", name, cacheEntry))
					issues = true
				}
			}
			report.PluginSources = append(report.PluginSources, buildItem(name, cacheEntry, hydrated, message))
		}

		for _, runtime := range cfg.TrunkConfig.Runtimes.Enabled {
			runtimeName := strings.TrimSpace(runtime)
			tool, version := splitToolReference(runtimeName)
			cacheEntry := ""
			hydrated := false
			message := ""
			if tool == "" || version == "" {
				message = "runtime entry missing version"
				report.Runtimes = append(report.Runtimes, toolHealthItem{Name: runtimeName, Status: "skipped", Message: message})
				continue
			}
			if cacheDir != "" {
				cacheEntry = cachePath(cacheDir, "runtimes", tool, version)
				hydrated = pathExists(cacheEntry)
				if !hydrated {
					message = "runtime cache not found"
					warnings = append(warnings, fmt.Sprintf("missing runtime cache %s (%s)", runtimeName, cacheEntry))
					issues = true
				}
			}
			report.Runtimes = append(report.Runtimes, buildItem(runtimeName, cacheEntry, hydrated, message))
		}

		for _, lint := range cfg.TrunkConfig.Lint.Enabled {
			lintName := strings.TrimSpace(lint)
			tool, version := splitToolReference(lintName)
			cacheEntry := ""
			hydrated := false
			message := ""
			if version == "" {
				report.Linters = append(report.Linters, toolHealthItem{Name: lintName, Status: "skipped", Message: "linter not pinned to a version"})
				continue
			}
			if cacheDir != "" {
				cacheEntry = cachePath(cacheDir, "tools", tool, version)
				hydrated = pathExists(cacheEntry)
				if !hydrated {
					message = "tool cache not found"
					warnings = append(warnings, fmt.Sprintf("missing tool cache %s (%s)", lintName, cacheEntry))
					issues = true
				}
			}
			report.Linters = append(report.Linters, buildItem(lintName, cacheEntry, hydrated, message))
		}
	}

	if len(warnings) > 0 {
		report.Warnings = append(report.Warnings, warnings...)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tool health: %w", err)
	}
	jsonText := string(data)
	jsonPath := strings.TrimSpace(cfg.ToolHealthJSONPath)
	if jsonPath != "" {
		if err := os.MkdirAll(filepath.Dir(jsonPath), 0o755); err != nil {
			return fmt.Errorf("ensure tool-health json directory: %w", err)
		}
		if err := os.WriteFile(jsonPath, []byte(jsonText), 0o644); err != nil {
			return fmt.Errorf("write tool-health json: %w", err)
		}
	}
	format := strings.TrimSpace(strings.ToLower(cfg.ToolHealthFormat))
	if format == "" {
		format = "json"
	}
	switch format {
	case "json":
		fmt.Println(jsonText)
	case "summary", "table":
		fmt.Println(renderToolHealthSummary(report))
	default:
		return fmt.Errorf("unsupported tool-health format %q", cfg.ToolHealthFormat)
	}
	if report.Trunk.Status == "mismatch" {
		issues = true
	}
	if !cacheAvailable && cacheDir != "" {
		issues = true
	}
	if issues {
		return fmt.Errorf("tool-health detected issues; see report warnings for details")
	}
	return nil
}

func renderToolHealthSummary(report toolHealthReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Tool Health Summary (%s)\n", report.Timestamp)
	if report.Trunk.Expected != "" {
		fmt.Fprintf(&b, "Trunk CLI: %s (expected %s, detected %s)", report.Trunk.Status, report.Trunk.Expected, report.Trunk.Detected)
	} else {
		fmt.Fprintf(&b, "Trunk CLI: %s", report.Trunk.Status)
		if report.Trunk.Detected != "" {
			fmt.Fprintf(&b, " (detected %s)", report.Trunk.Detected)
		}
	}
	if report.Trunk.Message != "" {
		fmt.Fprintf(&b, " - %s", report.Trunk.Message)
	}
	fmt.Fprintln(&b)
	if report.CacheDir != "" {
		fmt.Fprintf(&b, "Cache dir: %s\n", report.CacheDir)
	} else {
		fmt.Fprintln(&b, "Cache dir: (not resolved)")
	}
	appendItems := func(title string, items []toolHealthItem) {
		if len(items) == 0 {
			fmt.Fprintf(&b, "%s: none\n", title)
			return
		}
		fmt.Fprintf(&b, "%s:\n", title)
		for _, item := range items {
			fmt.Fprintf(&b, "  - %s: %s", item.Name, item.Status)
			if item.Message != "" {
				fmt.Fprintf(&b, " (%s)", item.Message)
			}
			if item.CachePath != "" {
				fmt.Fprintf(&b, " [%s]", item.CachePath)
			}
			fmt.Fprintln(&b)
		}
	}
	appendItems("Plugin sources", report.PluginSources)
	appendItems("Runtimes", report.Runtimes)
	appendItems("Linters", report.Linters)
	if len(report.Warnings) > 0 {
		fmt.Fprintln(&b, "Warnings:")
		for _, warning := range report.Warnings {
			fmt.Fprintf(&b, "  - %s\n", warning)
		}
	} else {
		fmt.Fprintln(&b, "Warnings: none")
	}
	return strings.TrimRight(b.String(), "\n")
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
	if len(sources) > 0 {
		var lastFailure DiagnoseCheck
		for _, src := range sources {
			resolved, err := resolveTrunkBinary(src)
			if err != nil {
				lastFailure = DiagnoseCheck{
					Name:           name,
					Status:         diagnoseStatusError,
					Message:        fmt.Sprintf("trunk binary %s is invalid: %v", src, err),
					Recommendation: "Provide a valid trunk executable via --trunk-binary or PUNCHTRUNK_TRUNK_BINARY.",
				}
				continue
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
		if lastFailure.Name != "" {
			return lastFailure
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
		logger := cfg.log()
		logger.Warnf("unable to clean up diagnostic file %s: %v", testFile, err)
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

var installTrunkFunc = installTrunk

func ensureTrunk(ctx context.Context, cfg *Config) (string, error) {
	logger := defaultLogger
	if cfg != nil {
		logger = cfg.log()
	}
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
			logger.Infof("Airgapped mode enabled; skipping Trunk auto-install.")
		}
		return "", fmt.Errorf("trunk executable not found and PUNCHTRUNK_AIRGAPPED is set. Provide --trunk-binary or install trunk manually in offline environments")
	}
	if cfg != nil && cfg.Verbose {
		logger.Infof("Trunk CLI not found in PATH. Attempting automatic install...")
	}
	if err := installTrunkFunc(ctx, cfg != nil && cfg.Verbose, logger); err != nil {
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

func installTrunk(ctx context.Context, verbose bool, logger *eventLogger) error {
	if logger == nil {
		logger = defaultLogger
	}
	switch runtime.GOOS {
	case "windows":
		return installTrunkWindows(ctx, verbose, logger)
	default:
		return installTrunkUnix(ctx, verbose, logger)
	}
}

func installTrunkUnix(ctx context.Context, verbose bool, logger *eventLogger) error {
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
			logger.Warnf("closing trunk installer response: %v", cerr)
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
			msg := fmt.Sprintf("removing trunk installer script %s: %v", tmpFile.Name(), removeErr)
			if verbose {
				logger.Warnf("%s", msg)
			} else {
				logger.Warnf("%s. Set TMPDIR to a writable location or clean up the file manually.", msg)
			}
		}
	}()
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		if closeErr := tmpFile.Close(); closeErr != nil && verbose {
			logger.Warnf("closing trunk installer temp file: %v", closeErr)
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

func installTrunkWindows(ctx context.Context, verbose bool, logger *eventLogger) error {
	if logger == nil {
		logger = defaultLogger
	}
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
	if err := cmd.Run(); err != nil {
		if !verbose {
			logger.Warnf("trunk installer failed: %v", err)
		}
		return err
	}
	return nil
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
			cfg.log().Warnf("unable to resolve changed files: %v", err)
		}
	} else {
		changed = m
		if degraded && cfg != nil && cfg.Verbose {
			cfg.log().Infof("falling back to limited git history for changed files; diff weighting may be incomplete")
		}
	}
	// Consider changed files as primary focus; also consider top churn files overall.
	churn, degradedChurn, err := gitChurn(ctx, "90 days")
	if err != nil {
		return nil, err
	}
	if degradedChurn && cfg != nil && cfg.Verbose {
		cfg.log().Infof("falling back to limited git history for churn; hotspot rankings may be partial")
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
		cfg.log().Infof("no git churn detected; hotspot report may be empty")
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
				cfg.log().Infof("%s failed: %v (%s)", att.desc, err, strings.TrimSpace(lastStderr))
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
