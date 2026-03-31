package harness

import (
	"strings"
	"testing"
)

func TestSanitizeKey_NormalKey(t *testing.T) {
	safe, err := sanitizeKey("my-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if safe != "my-key" {
		t.Errorf("expected 'my-key', got %q", safe)
	}
}

func TestSanitizeKey_TooLong(t *testing.T) {
	longKey := strings.Repeat("a", 201)
	_, err := sanitizeKey(longKey)
	if err == nil {
		t.Error("expected error for key > 200 chars")
	}
	if !strings.Contains(err.Error(), "too long") {
		t.Errorf("expected 'too long' in error, got: %v", err)
	}
}

func TestSanitizeKey_ExactlyMaxLength(t *testing.T) {
	key := strings.Repeat("a", 200)
	safe, err := sanitizeKey(key)
	if err != nil {
		t.Fatalf("expected no error at exactly 200 chars: %v", err)
	}
	if safe != key {
		t.Errorf("expected unchanged key at max length")
	}
}

func TestSanitizeKey_DotKey(t *testing.T) {
	_, err := sanitizeKey(".")
	if err == nil {
		t.Error("expected error for '.' key")
	}
}

func TestSanitizeKey_DotDotKey(t *testing.T) {
	_, err := sanitizeKey("..")
	if err == nil {
		t.Error("expected error for '..' key")
	}
}

func TestSanitizeKey_DotDotSlashPrefix(t *testing.T) {
	_, err := sanitizeKey("../etc/passwd")
	if err == nil {
		t.Error("expected error for '../...' prefix")
	}
}

func TestSanitizeKey_EmbeddedDotDotSlash(t *testing.T) {
	_, err := sanitizeKey("foo/../bar")
	if err == nil {
		t.Error("expected error for embedded /../")
	}
}

func TestSanitizeKey_SlashReplacedWithUnderscore(t *testing.T) {
	safe, err := sanitizeKey("path/to/key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(safe, "/") {
		t.Errorf("slash should be replaced, got %q", safe)
	}
}

func TestSanitizeKey_SpecialCharsReplaced(t *testing.T) {
	safe, err := sanitizeKey(`key:with*special?"<>|chars`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, bad := range []string{":", "*", "?", `"`, "<", ">", "|"} {
		if strings.Contains(safe, bad) {
			t.Errorf("char %q should be replaced in %q", bad, safe)
		}
	}
}

func TestSanitizeKey_NullByteReplaced(t *testing.T) {
	safe, err := sanitizeKey("key\x00name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(safe, "\x00") {
		t.Errorf("null byte should be replaced, got %q", safe)
	}
}

func TestSanitizeKey_DotReplacedWithUnderscore(t *testing.T) {
	safe, err := sanitizeKey("key.name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(safe, ".") {
		t.Errorf("dot should be replaced, got %q", safe)
	}
}

func TestSanitizeKey_BackslashReplaced(t *testing.T) {
	safe, err := sanitizeKey(`key\path`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(safe, `\`) {
		t.Errorf("backslash should be replaced, got %q", safe)
	}
}

func TestSanitizeKey_EmptyString_ReturnsError(t *testing.T) {
	_, err := sanitizeKey("")
	if err == nil {
		t.Error("expected error for empty key")
	}
}

func TestSanitizeKey_AllSpecialChars_AllReplaced_NotEmpty(t *testing.T) {
	// all replaced chars → becomes underscores, not empty
	safe, err := sanitizeKey("a/b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if safe == "" {
		t.Error("sanitized key should not be empty")
	}
}
