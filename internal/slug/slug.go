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

// BranchName builds a unique branch name with prompt slug, timestamp, and short guid.
func BranchName(prompt string, now time.Time, guid string) string {
	slug := FromPrompt(prompt)
	shortGUID := guid
	if len(shortGUID) > 8 {
		shortGUID = shortGUID[:8]
	}
	if shortGUID == "" {
		shortGUID = "noguid"
	}
	return fmt.Sprintf("codex/%s-%s-%s", slug, now.UTC().Format("20060102-150405"), shortGUID)
}
