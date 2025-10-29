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
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Modes      []string
	Autofix    string
	BaseBranch string
	MaxProcs   int
	Timeout    time.Duration
	SarifOut   string
	Verbose    bool
}

func main() {
	cfg := parseFlags()
	if cfg.MaxProcs <= 0 {
		cfg.MaxProcs = runtime.NumCPU()
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	modes := make(map[string]bool)
	for _, m := range cfg.Modes {
		modes[strings.ToLower(strings.TrimSpace(m))] = true
	}

	// Ensure reports dir
	_ = os.MkdirAll("reports", 0o755)

	// 1) fmt (formatters only)
	if modes["fmt"] {
		if err := runTrunkFmt(ctx, cfg); err != nil {
			log.Printf("WARN: trunk fmt failed: %v", err)
		}
	}

	// 2) lint (linters through trunk check)
	if modes["lint"] {
		if err := runTrunkCheck(ctx, cfg); err != nil {
			log.Printf("WARN: trunk check failed: %v", err)
		}
	}

	// 3) hotspots -> SARIF
	var hs []Hotspot
	if modes["hotspots"] {
		var err error
		hs, err = computeHotspots(ctx, cfg)
		if err != nil {
			log.Printf("WARN: hotspot computation failed: %v", err)
		} else {
			if err := writeSARIF(cfg.SarifOut, hs); err != nil {
				log.Printf("WARN: writing SARIF failed: %v", err)
			} else if cfg.Verbose {
				log.Printf("Wrote SARIF to %s (%d results)", cfg.SarifOut, len(hs))
			}
		}
	}

	// Exit code policy: non-zero if trunk check returned non-zero.
	// We can't perfectly detect here without parsing, so we surface best-effort:
	// - If lint ran, and trunk check returned non-zero, we already printed warning;
	//   we'll propagate a non-zero exit for CI strictness.
	if exitErr != nil {
		os.Exit(1)
	}
}

// Global to capture trunk check exit for CI policy.
var exitErr error

func parseFlags() *Config {
	var modes string
	var base string
	var maxProcs int
	var timeoutSec int
	var sarifOut string
	var verbose bool
	var autofix string
	flag.StringVar(&modes, "mode", "fmt,lint,hotspots", "Comma-separated phases: fmt,lint,hotspots")
	flag.StringVar(&autofix, "autofix", "fmt", "Autofix scope: none|fmt|lint|all")
	flag.StringVar(&base, "base-branch", "origin/main", "Base branch for change detection")
	flag.IntVar(&maxProcs, "max-procs", 0, "Parallelism cap (0 = CPU cores)")
	flag.IntVar(&timeoutSec, "timeout", 900, "Overall timeout in seconds")
	flag.StringVar(&sarifOut, "sarif-out", "reports/hotspots.sarif", "SARIF output path for hotspots")
	flag.BoolVar(&verbose, "verbose", false, "Verbose logs")
	flag.Parse()
	return &Config{
		Modes:      splitCSV(modes),
		Autofix:    strings.ToLower(autofix),
		BaseBranch: base,
		MaxProcs:   maxProcs,
		Timeout:    time.Duration(timeoutSec) * time.Second,
		SarifOut:   sarifOut,
		Verbose:    verbose,
	}
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
	args := []string{"fmt"}
	cmd := exec.CommandContext(ctx, "trunk", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if cfg.Verbose {
		log.Printf("Running: trunk %s", strings.Join(args, " "))
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
	// Let trunk decide changed files via hold-the-line; base branch is read from trunk.yaml.
	cmd := exec.CommandContext(ctx, "trunk", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if cfg.Verbose {
		log.Printf("Running: trunk %s", strings.Join(args, " "))
	}
	err := cmd.Run()
	if err != nil {
		exitErr = err
	}
	return err
}

type Hotspot struct {
	File       string
	Churn      int
	Complexity float64
	Score      float64
}

func computeHotspots(ctx context.Context, cfg *Config) ([]Hotspot, error) {
	changed, _ := gitChangedFiles(ctx, cfg.BaseBranch)
	// Consider changed files as primary focus; also consider top churn files overall.
	churn, err := gitChurn(ctx, "90 days")
	if err != nil {
		return nil, err
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

func gitChangedFiles(ctx context.Context, base string) (map[string]bool, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", base+"...HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
	lines := strings.Split(out.String(), "\n")
	m := map[string]bool{}
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			m[l] = true
		}
	}
	return m, nil
}

func gitChurn(ctx context.Context, since string) (map[string]int, error) {
	cmd := exec.CommandContext(ctx, "git", "log", fmt.Sprintf("--since=%s", since), "--numstat", "--format=tformat:")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	churn := map[string]int{}
	for _, line := range strings.Split(out.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 {
			added := fields[0]
			deleted := fields[1]
			file := fields[2]
			if added == "-" || deleted == "-" {
				// binary; count as 1 change
				churn[file] += 1
				continue
			}
			a := atoiSafe(added)
			d := atoiSafe(deleted)
			churn[file] += a + d
		}
	}
	return churn, nil
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
