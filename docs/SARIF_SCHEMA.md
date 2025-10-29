# SARIF Schema Documentation

## Overview

PunchTrunk generates SARIF (Static Analysis Results Interchange Format) files for hotspot analysis. This document describes the schema, structure, and expectations for downstream automation.

## SARIF Version

PunchTrunk outputs **SARIF 2.1.0** format, conforming to the [OASIS SARIF 2.1.0 specification](https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html).

## Schema Location

```text
https://schemastore.azurewebsites.net/schemas/json/sarif-2.1.0-rtm.5.json
```

## Output Structure

### File Location

By default, SARIF files are written to:

```text
reports/hotspots.sarif
```

If the checkout is read-only, PunchTrunk automatically redirects output to:

```text
/tmp/punchtrunk/reports/<filename>
```

The CLI emits a log entry with the exact fallback path so CI upload steps can adjust.

This can be customized with the `--sarif-out` flag:

```bash
./bin/punchtrunk --mode hotspots --sarif-out custom/path/output.sarif
```

### Basic Structure

```json
{
  "version": "2.1.0",
  "$schema": "https://schemastore.azurewebsites.net/schemas/json/sarif-2.1.0-rtm.5.json",
  "runs": [
    {
      "tool": {
        "driver": {
          "name": "PunchTrunk",
          "informationUri": "https://docs.trunk.io/"
        }
      },
      "results": [...]
    }
  ]
}
```

## Result Format

Each hotspot is represented as a SARIF result:

```json
{
  "ruleId": "hotspot",
  "level": "note",
  "message": {
    "text": "Hotspot candidate: churn=42, complexity=3.14, score=7.89"
  },
  "locations": [
    {
      "physicalLocation": {
        "artifactLocation": {
          "uri": "path/to/file.go"
        }
      }
    }
  ]
}
```

### Field Descriptions

#### ruleId

- **Value**: `"hotspot"`
- **Type**: Fixed identifier for all hotspot results
- **Purpose**: Groups all hotspot findings under a single rule

#### level

- **Value**: `"note"`
- **Type**: Severity level
- **Rationale**: Hotspots are informational, not bugs or vulnerabilities
- **Options**: `"error"`, `"warning"`, `"note"`, `"none"`

#### message.text

- **Format**: `"Hotspot candidate: churn={int}, complexity={float}, score={float}"`
- **Components**:
  - `churn`: Number of lines added/modified in the last 90 days
  - `complexity`: Token-per-line ratio (proxy for code complexity)
  - `score`: Composite ranking score `log(1 + churn) * (1 + complexity_z)`
- **Example**: `"Hotspot candidate: churn=42, complexity=3.14, score=7.89"`

#### locations[0].physicalLocation.artifactLocation.uri

- **Format**: Relative file path from repository root
- **Path Separator**: Forward slash (`/`) for cross-platform compatibility
- **Example**: `"cmd/punchtrunk/main.go"`

## Hotspot Ranking

Results are ordered by **descending score**, with the highest-scoring (riskiest) files first:

1. Higher churn → higher risk
2. Higher complexity → higher risk
3. Changed files get 1.15× score multiplier
4. Limited to top 500 hotspots for performance

### Score Formula

```text
score = log(1 + churn) × (1 + complexity_z)
```

Where:

- `churn`: Total lines changed in the analysis window
- `complexity_z`: Z-score normalized complexity across all files
- Changed files: Multiplied by 1.15

## Downstream Integration

### GitHub Code Scanning

Upload SARIF to GitHub Code Scanning using the CodeQL action:

```yaml
- name: Upload SARIF
  uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: reports/hotspots.sarif
```

Results appear in:

- Repository **Security** tab → **Code scanning**
- Pull request annotations (if configured)

### Validation

Validate SARIF output before upload:

```bash
# Check valid JSON
jq empty reports/hotspots.sarif

# Validate version
jq -r '.version' reports/hotspots.sarif  # Should output: 2.1.0

# Validate tool name
jq -r '.runs[0].tool.driver.name' reports/hotspots.sarif  # Should output: PunchTrunk

# Count results
jq '.runs[0].results | length' reports/hotspots.sarif
```

### Custom Processing

Example: Extract top 10 hotspots:

```bash
jq '.runs[0].results[:10] | .[] | {
  file: .locations[0].physicalLocation.artifactLocation.uri,
  message: .message.text
}' reports/hotspots.sarif
```

Example: Filter by score threshold:

```bash
jq '.runs[0].results[] | select(.message.text | contains("score=") |
  split("score=")[1] | split(")")[0] | tonumber > 5.0)' reports/hotspots.sarif
```

## Quality Assurance

### Schema Compliance

PunchTrunk's SARIF output is tested for:

1. **Valid JSON**: No syntax errors
2. **Schema version**: Exactly `"2.1.0"`
3. **Required fields**: All mandatory SARIF fields present
4. **Tool metadata**: Correct tool name and URI
5. **Result structure**: Proper nesting and field types

### Automated Tests

E2E tests validate SARIF generation:

```bash
# Kitchen sink test includes SARIF validation
go test -v ./cmd/punchtrunk -run TestE2EKitchenSink

# Integration test with actual upload
# See .github/workflows/e2e.yml
```

### Manual Validation

```bash
# Generate SARIF
./bin/punchtrunk --mode hotspots

# Validate with online tools
# 1. https://sarifweb.azurewebsites.net/Validation
# 2. Upload reports/hotspots.sarif
# 3. Verify no errors
```

## Customization

### Churn Window

Currently fixed at 90 days. Future enhancement:

```bash
# Future: Custom churn window (not yet implemented)
./bin/punchtrunk --mode hotspots --churn-window "180 days"
```

See [ROADMAP.md](ROADMAP.md) for planned features.

### Score Tuning

Modify score calculation in `cmd/punchtrunk/main.go`:

```go
// Current formula
score := math.Log1p(float64(ch)) * (1.0 + cz)

// Example: Emphasize complexity more
score := math.Log1p(float64(ch)) * (1.0 + 2.0*cz)

// Example: Linear churn instead of logarithmic
score := float64(ch) * (1.0 + cz)
```

### Result Limit

Currently capped at 500 results. Modify in `computeHotspots()`:

```go
// Current limit
if len(hs) > 500 {
    hs = hs[:500]
}

// Custom limit
if len(hs) > 1000 {
    hs = hs[:1000]
}
```

## Troubleshooting

### Empty SARIF File

**Problem**: No results in SARIF output

**Causes**:

1. No git history (shallow clone without `fetch-depth: 0`)
2. No file changes in the analysis window
3. All files excluded by gitignore

**Solution**:

```bash
# Check git history depth
git log --oneline | wc -l  # Should be > 1

# Verify file changes
git log --since="90 days ago" --numstat

# Run with verbose logging
./bin/punchtrunk --mode hotspots --verbose
```

### Invalid SARIF

**Problem**: GitHub Code Scanning rejects SARIF

**Solution**:

```bash
# Validate JSON syntax
jq empty reports/hotspots.sarif || echo "Invalid JSON"

# Check required fields
jq '{
  version: .version,
  toolName: .runs[0].tool.driver.name,
  resultCount: .runs[0].results | length
}' reports/hotspots.sarif
```

### Missing File Paths

**Problem**: File paths not recognized by GitHub

**Causes**:

1. Absolute paths instead of relative
2. Wrong path separator (backslash on Windows)

**Solution**: PunchTrunk uses `filepath.ToSlash()` to normalize paths. If issues persist, check:

```bash
# Verify paths are relative
jq -r '.runs[0].results[].locations[0].physicalLocation.artifactLocation.uri' \
  reports/hotspots.sarif | head -5

# Should output: relative/path/to/file.go
# NOT: /absolute/path or C:\Windows\path
```

## References

### Specifications

- [SARIF 2.1.0 Specification](https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html)
- [SARIF Tutorials](https://github.com/microsoft/sarif-tutorials)

### GitHub Integration

- [Code Scanning SARIF Support](https://docs.github.com/en/code-security/code-scanning/integrating-with-code-scanning/sarif-support-for-code-scanning)
- [Uploading SARIF Files](https://docs.github.com/en/code-security/code-scanning/integrating-with-code-scanning/uploading-a-sarif-file-to-github)

### Tools

- [SARIF Viewer (VS Code)](https://marketplace.visualstudio.com/items?itemName=MS-SarifVSCode.sarif-viewer)
- [SARIF Web Validator](https://sarifweb.azurewebsites.net/)
- [jq Manual](https://stedolan.github.io/jq/manual/)

## Changelog

### Current Version

- SARIF 2.1.0 compliant
- File-level hotspot results
- Note severity level
- Top 500 results cap
- 90-day churn window

### Future Enhancements

See [ROADMAP.md](ROADMAP.md):

- Custom churn windows
- CODEOWNERS integration
- Additional metadata fields
- Performance optimizations
