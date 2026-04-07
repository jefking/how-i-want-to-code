package slug

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// FromPrompt converts prompt text into a branch-safe slug.
func FromPrompt(prompt string) string {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	if lower == "" {
		return "task"
	}
	s := nonAlnum.ReplaceAllString(lower, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "task"
	}
	if len(s) > 40 {
		s = s[:40]
		s = strings.Trim(s, "-")
	}
	if s == "" {
		return "task"
	}
	return s
}

// BranchName builds a branch name with a stable prompt slug.
func BranchName(prompt string, now time.Time, guid string) string {
	slug := FromPrompt(prompt)
	_ = now
	_ = guid
	return fmt.Sprintf("moltenhub-%s", slug)
}
