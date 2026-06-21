package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func TestDocumentationInventoryContainsCoreFiles(t *testing.T) {
	for _, path := range []string{
		"README.md",
		"AGENTS.md",
		"CLAUDE.md",
		"docs/README.md",
		"docs/architecture.md",
		"docs/benchmarks.md",
		"docs/design.md",
		"docs/operations.md",
		"docs/requirements.md",
		"docs/test-and-benchmark-gaps.md",
		".pi/agents/software-designer.md",
		".pi/agents/software-developer.md",
		".pi/agents/software-implementer.md",
		".pi/agents/software-qa.md",
		".pi/agents/software-reviewer.md",
		".pi/agents/software-systems-engineer.md",
		".pi/agents/user-representative.md",
		".pi/prompts/software-developer-parallel.md",
		".pi/prompts/software-role-workflow.md",
		".pi/skills/software-role-agents/SKILL.md",
		".pi/skills/software-developer-parallel/SKILL.md",
		".pi/skills/iterative-findings-loop/SKILL.md",
		".pi/skills/run-cvmm-validation/SKILL.md",
		".pi/extensions/guardrails.json",
		".pi/extensions/guardrails.v0.json",
		".pi/settings.json",
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing documentation inventory entry %q: %v", path, err)
		}
	}
}

func TestMarkdownLocalLinksResolve(t *testing.T) {
	linkPattern := regexp.MustCompile(`!?\[[^\]]*\]\(([^)]+)\)`)
	for _, path := range markdownFiles(t) {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		for _, match := range linkPattern.FindAllStringSubmatch(string(content), -1) {
			target := normalizeMarkdownTarget(match[1])
			if target == "" {
				continue
			}
			resolved := target
			if !filepath.IsAbs(target) {
				resolved = filepath.Clean(filepath.Join(filepath.Dir(path), target))
			}
			if _, err := os.Stat(resolved); err != nil {
				t.Fatalf("broken markdown link in %s: %q -> %s (%v)", path, match[1], resolved, err)
			}
		}
	}
}

func TestDocumentationCorrectnessGuardrails(t *testing.T) {
	bannedProjectIdentities := []string{
		"fast-volume-syncer",
		"holefs",
		"init-wrapper",
		"screenfs",
	}
	for _, path := range append(markdownFiles(t), mermaidFiles(t)...) {
		if strings.HasPrefix(path, "docs/guidelines"+string(filepath.Separator)) {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		lower := strings.ToLower(string(content))
		for _, banned := range bannedProjectIdentities {
			if strings.Contains(lower, banned) {
				t.Fatalf("%s contains legacy project identity %q; active cvmm docs must not inherit unrelated project claims", path, banned)
			}
		}
		assertNoUnavailableEvidenceClaims(t, path, string(content))
	}
}

func TestPiGuardrailsConfigMatchesDocumentation(t *testing.T) {
	type guardrailsConfig struct {
		Schema     string `json:"$schema"`
		Version    string `json:"version"`
		PathAccess *struct {
			AllowedPaths *[]json.RawMessage `json:"allowedPaths"`
		} `json:"pathAccess"`
	}
	type allowedPath struct {
		Kind string `json:"kind"`
		Path string `json:"path"`
	}

	for _, path := range []string{".pi/extensions/guardrails.json", ".pi/extensions/guardrails.v0.json"} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		var cfg guardrailsConfig
		if err := json.Unmarshal(content, &cfg); err != nil {
			t.Fatalf("json.Unmarshal(%q) error = %v", path, err)
		}
		if cfg.Schema == "" {
			t.Fatalf("%s must keep $schema so Pi guardrail shape remains explicit", path)
		}
		if cfg.PathAccess == nil {
			t.Fatalf("%s must keep pathAccess so Pi guardrail shape remains explicit", path)
		}
		if cfg.PathAccess.AllowedPaths == nil {
			t.Fatalf("%s must keep pathAccess.allowedPaths so project-local path exceptions remain explicit", path)
		}
		if path == ".pi/extensions/guardrails.v0.json" {
			if len(*cfg.PathAccess.AllowedPaths) != 0 {
				t.Fatalf("%s pathAccess.allowedPaths = %s, want empty legacy path exception list", path, content)
			}
			continue
		}
		if cfg.Version == "" {
			t.Fatalf("current guardrails.json must keep version documented in docs/pi-agents.md")
		}
		want := []allowedPath{
			{Kind: "directory", Path: "/srv/vmm"},
			{Kind: "directory", Path: "/dev"},
		}
		if len(*cfg.PathAccess.AllowedPaths) != len(want) {
			t.Fatalf("%s pathAccess.allowedPaths length = %d, want %d", path, len(*cfg.PathAccess.AllowedPaths), len(want))
		}
		for idx, raw := range *cfg.PathAccess.AllowedPaths {
			var got allowedPath
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("%s pathAccess.allowedPaths[%d] error = %v", path, idx, err)
			}
			if got != want[idx] {
				t.Fatalf("%s pathAccess.allowedPaths[%d] = %#v, want %#v", path, idx, got, want[idx])
			}
		}
	}
}

func assertNoUnavailableEvidenceClaims(t *testing.T, path, content string) {
	t.Helper()
	for lineNo, line := range strings.Split(content, "\n") {
		lower := strings.ToLower(line)
		if mentionsUnavailableEvidenceSource(lower) && !containsEvidenceNegation(lower) {
			t.Fatalf("%s:%d appears to claim unavailable legacy artifact/benchmark evidence as current cvmm evidence: %s", path, lineNo+1, strings.TrimSpace(line))
		}
	}
}

func mentionsUnavailableEvidenceSource(lower string) bool {
	legacyArtifact := strings.Contains(lower, "legacy non-cvmm artifact") && (strings.Contains(lower, "evidence") || strings.Contains(lower, "근거"))
	benchmarkHarness := strings.Contains(lower, "benchmark harness") && containsEvidenceClaimVerb(lower)
	return legacyArtifact || benchmarkHarness
}

func containsEvidenceClaimVerb(lower string) bool {
	for _, claimVerb := range []string{"exists", "available", "present", "implemented", "제공", "구현", "있다", "있음", "존재", "사용 가능", "완료"} {
		if strings.Contains(lower, claimVerb) {
			return true
		}
	}
	return false
}

func containsEvidenceNegation(lower string) bool {
	negationPattern := regexp.MustCompile(`\bnot\b|\bno\b|없|부재|아직|비목표|제거|않|아니|재사용하지|섞지|대체하지|별도 작업|분리`)
	return negationPattern.MatchString(lower)
}

func markdownFiles(t *testing.T) []string {
	t.Helper()
	files := []string{}
	for _, root := range []string{".", "docs", ".pi"} {
		if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if root == "." && (path == ".git" || path == "docs" || path == ".pi") {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(strings.ToLower(path), ".md") {
				files = append(files, path)
			}
			return nil
		}); err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}
	slices.Sort(files)
	return files
}

func mermaidFiles(t *testing.T) []string {
	t.Helper()
	files := []string{}
	if err := filepath.WalkDir("docs/diagrams", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(path), ".mmd") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk docs/diagrams: %v", err)
	}
	slices.Sort(files)
	return files
}

func normalizeMarkdownTarget(raw string) string {
	target := strings.TrimSpace(raw)
	if idx := strings.Index(target, " "); idx >= 0 {
		target = target[:idx]
	}
	target = strings.Trim(target, "<>")
	if target == "" || strings.HasPrefix(target, "#") {
		return ""
	}
	for _, prefix := range []string{"http://", "https://", "mailto:", "data:"} {
		if strings.HasPrefix(target, prefix) {
			return ""
		}
	}
	if idx := strings.Index(target, "#"); idx >= 0 {
		target = target[:idx]
	}
	return target
}
