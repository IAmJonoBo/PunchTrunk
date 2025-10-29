package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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
		name     string
		content  string
		wantMin  float64
		wantMax  float64
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
