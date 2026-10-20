package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("# comment\nJOB_INBOX_TEST_VALUE=loaded\nQUOTED=\"hello world\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JOB_INBOX_TEST_VALUE", "existing")
	if err := loadFile(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("JOB_INBOX_TEST_VALUE"); got != "existing" {
		t.Fatalf("existing environment variable was replaced: %q", got)
	}
	if got := os.Getenv("QUOTED"); got != "hello world" {
		t.Fatalf("quoted value = %q", got)
	}
}
