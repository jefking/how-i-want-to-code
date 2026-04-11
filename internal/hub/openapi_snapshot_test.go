package hub

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var requiredRuntimeIntegrationRoutes = []string{
	"/agents/bind:",
	"/agents/me/metadata:",
	"/agents/me:",
	"/messages/publish:",
	"/messages/pull:",
	"/messages/ack:",
	"/messages/nack:",
	"/openclaw/messages/publish:",
	"/openclaw/messages/pull:",
	"/openclaw/messages/ack:",
	"/openclaw/messages/nack:",
	"/openclaw/messages/ws:",
	"/openclaw/messages/offline:",
}

func TestOpenAPISnapshotIncludesRuntimeIntegrationRoutes(t *testing.T) {
	t.Parallel()

	content, source, err := loadOpenAPIContractForTest()
	if err != nil {
		t.Skipf("skipping OpenAPI route validation: %v", err)
	}
	t.Logf("validated OpenAPI contract from %s", source)

	assertOpenAPIRoutes(t, source, content)
	if !strings.Contains(content, "/messages/pull:\n    get:") {
		t.Fatalf("%s missing expected GET method for /messages/pull", source)
	}

	if strings.Contains(content, "/openclaw/messages/register-plugin:") {
		t.Fatalf("%s unexpectedly contains undocumented route /openclaw/messages/register-plugin", source)
	}

	bindSection := openAPIPathBlock(content, "/v1/agents/bind")
	if bindSection == "" {
		t.Fatalf("%s missing expected bind route section", source)
	}
	if !strings.Contains(bindSection, "required: [bind_token]") {
		t.Fatalf("%s bind route missing bind_token requirement", source)
	}
	if strings.Contains(bindSection, "humanAuth") {
		t.Fatalf("%s bind route should not require human auth", source)
	}

	bindTokensSection := openAPIPathBlock(content, "/v1/agents/bind-tokens")
	if bindTokensSection == "" {
		t.Fatalf("%s missing expected bind-tokens route section", source)
	}
	if !strings.Contains(bindTokensSection, "humanAuth") {
		t.Fatalf("%s bind-tokens route missing human auth requirement", source)
	}
}

func TestOpenAPISnapshotFileExistsForOfflineReview(t *testing.T) {
	t.Parallel()

	content, source, err := loadOpenAPIContractForTest()
	if err != nil {
		t.Fatal(err)
	}
	assertOpenAPIRoutes(t, source, content)
}

func loadOpenAPIContractForTest() (content string, source string, err error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", "", fmt.Errorf("runtime.Caller(0) failed")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	snapshotPath := filepath.Join(repoRoot, "na.hub.molten.bot.openapi.yaml")
	data, readErr := os.ReadFile(snapshotPath)
	if readErr != nil {
		return "", "", fmt.Errorf("read OpenAPI snapshot %s: %w", snapshotPath, readErr)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return "", "", fmt.Errorf("OpenAPI snapshot %s is empty", snapshotPath)
	}
	return string(data), snapshotPath, nil
}

func assertOpenAPIRoutes(t *testing.T, source, content string) {
	t.Helper()
	for _, route := range requiredRuntimeIntegrationRoutes {
		if !strings.Contains(content, route) {
			t.Fatalf("%s missing required route %q", source, route)
		}
	}
}

func openAPIPathBlock(content, path string) string {
	content = strings.TrimSpace(content)
	path = strings.TrimSpace(path)
	if content == "" || path == "" {
		return ""
	}

	marker := "  " + path + ":"
	start := strings.Index(content, marker)
	if start < 0 {
		return ""
	}

	rest := content[start+len(marker):]
	next := strings.Index(rest, "\n  /")
	if next < 0 {
		return content[start:]
	}
	return content[start : start+len(marker)+next]
}
