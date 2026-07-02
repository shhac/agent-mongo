// Package skillmeta holds repo-level checks on the Claude Code skill files
// (port of the TS skill-metadata test).
package skillmeta

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

const maxDescriptionLength = 1024

func TestSkillFrontmatterWithinHarnessLimits(t *testing.T) {
	data, err := os.ReadFile("../../skills/agent-mongo/SKILL.md")
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}

	match := regexp.MustCompile(`(?s)^---\n(.*?)\n---\n`).FindSubmatch(data)
	if match == nil {
		t.Fatal("SKILL.md missing YAML frontmatter")
	}
	frontmatter := string(match[1])

	fields := map[string]string{}
	var current string
	for _, line := range strings.Split(frontmatter, "\n") {
		if key, value, found := strings.Cut(line, ":"); found && !strings.HasPrefix(line, " ") {
			current = strings.TrimSpace(key)
			fields[current] = strings.TrimSpace(value)
			continue
		}
		// Continuation lines (folded/multi-line YAML scalars) extend the value.
		if current != "" {
			fields[current] += " " + strings.TrimSpace(line)
		}
	}

	if name := strings.TrimSpace(strings.Trim(fields["name"], `"'`)); name != "agent-mongo" {
		t.Errorf("frontmatter name: got %q, want \"agent-mongo\"", name)
	}
	description := strings.TrimSpace(fields["description"])
	if description == "" || description == ">-" || description == "|" {
		description = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(description, ">-"), "|"))
	}
	if description == "" {
		t.Error("frontmatter description missing")
	}
	if len(description) > maxDescriptionLength {
		t.Errorf("frontmatter description %d chars exceeds harness limit %d",
			len(description), maxDescriptionLength)
	}
}
