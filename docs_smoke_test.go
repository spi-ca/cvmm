package main

import (
	"os"
	"path/filepath"
	"regexp"
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
		".pi/agents/software-developer.md",
		".pi/prompts/software-role-workflow.md",
		".pi/skills/software-role-agents/SKILL.md",
		".pi/skills/software-developer-parallel/SKILL.md",
		".pi/skills/iterative-findings-loop/SKILL.md",
		".pi/skills/run-cvmm-validation/SKILL.md",
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing documentation inventory entry %q: %v", path, err)
		}
	}
}

func TestMarkdownLocalLinksResolve(t *testing.T) {
	markdownFiles := []string{}
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
				markdownFiles = append(markdownFiles, path)
			}
			return nil
		}); err != nil {
			t.Fatalf("walk %s: %v", root, err)
		}
	}

	linkPattern := regexp.MustCompile(`!?\[[^\]]*\]\(([^)]+)\)`)
	for _, path := range markdownFiles {
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
