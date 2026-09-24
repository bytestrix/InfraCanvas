package redactor

import (
	"strings"
	"testing"
)

const placeholder = "[REDACTED]"

func TestRedactValueByKeyName(t *testing.T) {
	r := NewRedactor(true)
	for _, key := range []string{"DB_PASSWORD", "GITHUB_TOKEN", "AWS_SECRET_ACCESS_KEY", "api_key", "Authorization", "MYSQL_PWD"} {
		if got := r.RedactValue(key, "hunter2"); got != placeholder {
			t.Errorf("%s: got %q, want redacted", key, got)
		}
	}
	for _, key := range []string{"PATH", "HOME", "NODE_ENV", "PORT"} {
		if got := r.RedactValue(key, "value"); got != "value" {
			t.Errorf("%s: got %q, want unchanged", key, got)
		}
	}
}

func TestRedactValueByPattern(t *testing.T) {
	r := NewRedactor(true)
	cases := map[string]string{
		"jwt":         "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
		"private key": "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBg\n-----END PRIVATE KEY-----",
	}
	for name, v := range cases {
		if got := r.RedactValue("SOME_VAR", v); got != placeholder {
			t.Errorf("%s not redacted: %q", name, got)
		}
	}
}

func TestRedactCommandLine(t *testing.T) {
	r := NewRedactor(true)
	got := r.RedactCommandLine("postgres --password=hunter2 --port 5432 --token abc123")
	if strings.Contains(got, "hunter2") || strings.Contains(got, "abc123") {
		t.Fatalf("secret left in command line: %q", got)
	}
	if !strings.Contains(got, "--port 5432") {
		t.Fatalf("non-secret flag was changed: %q", got)
	}
}

func TestDisabledRedactorPassesThrough(t *testing.T) {
	r := NewRedactor(false)
	if got := r.RedactValue("DB_PASSWORD", "hunter2"); got != "hunter2" {
		t.Fatalf("disabled redactor changed value: %q", got)
	}
	env := map[string]string{"DB_PASSWORD": "x"}
	if got := r.RedactEnvVars(env); got["DB_PASSWORD"] != "x" {
		t.Fatalf("disabled redactor changed env: %v", got)
	}
}
