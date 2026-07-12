package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func isolate(t *testing.T) {
	t.Helper()
	for _, k := range []string{"SQUARE_ACCESS_TOKEN", "SQUARE_ENVIRONMENT", "SQUARE_VERSION", "SQUARE_BASE_URL", "SQUARE_PROFILE", "XDG_CONFIG_HOME"} {
		t.Setenv(k, "")
	}
	t.Setenv("SQUARE_CONFIG", filepath.Join(t.TempDir(), "missing.yaml"))
}

func TestRootBuildsCommandTree(t *testing.T) {
	root := NewRootCommand()
	groups := 0
	for _, c := range root.Commands() {
		if c.Name() == "config" || c.Name() == "help" || c.Name() == "completion" {
			continue
		}
		groups++
	}
	// One command group per Square API namespace (43 at time of writing).
	if groups < 40 {
		t.Errorf("expected >=40 API command groups, got %d", groups)
	}
}

func TestDocsFlagPrintsReferenceURL(t *testing.T) {
	isolate(t)
	root := NewRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"payments", "create-payment", "--docs"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := strings.TrimSpace(out.String())
	want := "https://developer.squareup.com/reference/square/payments-api/create-payment"
	if got != want {
		t.Errorf("docs URL = %q, want %q", got, want)
	}
}

func TestInvalidBodyRejectedBeforeRequest(t *testing.T) {
	isolate(t)
	root := NewRootCommand()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	// Unknown field should be rejected by client-side validation, so the missing
	// access token is never reached.
	root.SetArgs([]string{"payments", "create-payment", "--access-token", "x", "--body", `{"nope":1}`})
	err := root.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected validation error, got %v", err)
	}
}
