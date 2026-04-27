package privacy

import (
	"strings"
	"testing"
)

func TestFilter_Scrub(t *testing.T) {
	filter := NewFilter()

	tests := []struct {
		name       string
		input      string
		wantClean  bool
		wantRedact int
		mustNotContain string
	}{
		{
			name:       "OpenAI API key",
			input:      "export OPENAI_API_KEY=sk-proj-abc123def456ghi789jkl012mno345pqr678",
			wantClean:  false,
			wantRedact: 1,
			mustNotContain: "sk-proj",
		},
		{
			name:       "GitHub token",
			input:      "token: ghp_1234567890abcdefghijklmnopqrstuvwxyz12",
			wantClean:  false,
			wantRedact: 1,
			mustNotContain: "ghp_",
		},
		{
			name:       "AWS access key",
			input:      "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE",
			wantClean:  false,
			wantRedact: 1,
			mustNotContain: "AKIA",
		},
		{
			name:       "Postgres connection string",
			input:      "DATABASE_URL=postgres://user:password@host:5432/dbname",
			wantClean:  false,
			wantRedact: 1,
			mustNotContain: "postgres://",
		},
		{
			name:       "JWT token",
			input:      "auth: eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U",
			wantClean:  false,
			wantRedact: 1,
			mustNotContain: "eyJ",
		},
		{
			name:       "Stripe key",
			input:      "sk_test_51J0ABcDeFGhIjKlMnOpQrStUvWxYz",
			wantClean:  false,
			wantRedact: 1,
			mustNotContain: "sk_test_",
		},
		{
			name:       "Clean content",
			input:      "This is a normal paragraph with no secrets.",
			wantClean:  true,
			wantRedact: 0,
		},
		{
			name:       "Password in config",
			input:      `password = "my_super_secret_password123"`,
			wantClean:  false,
			wantRedact: 1,
			mustNotContain: "my_super_secret",
		},
		{
			name:       "Private key PEM",
			input:      "-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----",
			wantClean:  false,
			wantRedact: 1,
			mustNotContain: "BEGIN RSA PRIVATE KEY",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scrubbed, count := filter.Scrub(tt.input)

			if tt.wantClean && count > 0 {
				t.Errorf("expected clean content, got %d redactions", count)
			}
			if !tt.wantClean && count < tt.wantRedact {
				t.Errorf("expected at least %d redaction(s), got %d (output: %s)", tt.wantRedact, count, scrubbed)
			}
			if tt.mustNotContain != "" && strings.Contains(scrubbed, tt.mustNotContain) {
				t.Errorf("scrubbed output still contains %q: %s", tt.mustNotContain, scrubbed)
			}
		})
	}
}

func TestFilter_ContainsSecrets(t *testing.T) {
	filter := NewFilter()

	if filter.ContainsSecrets("hello world") {
		t.Error("expected no secrets in clean text")
	}

	if !filter.ContainsSecrets("sk-proj-abc123def456ghi789jkl012") {
		t.Error("expected OpenAI key to be detected")
	}
}

func TestFilter_DetectedSecrets(t *testing.T) {
	filter := NewFilter()

	secrets := filter.DetectedSecrets("key: sk-proj-abc123def456ghi789jkl012 and ghp_1234567890abcdefghijklmnopqrstuvwxyz12")
	if len(secrets) < 2 {
		t.Errorf("expected at least 2 secret types, got %d: %v", len(secrets), secrets)
	}
}

func TestFilter_ScrubLines(t *testing.T) {
	filter := NewFilter()

	input := "line1: clean\nline2: sk-proj-abc123def456ghi789jkl012\nline3: clean"
	scrubbed, count := filter.ScrubLines(input)

	if count == 0 {
		t.Error("expected redactions in multi-line input")
	}

	lines := strings.Split(scrubbed, "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}

	if strings.Contains(lines[1], "sk-proj") {
		t.Error("secret not removed from line 2")
	}
}

func TestFilter_AddCustomPattern(t *testing.T) {
	filter := NewFilter()

	err := filter.AddCustomPattern("custom_token", `CUSTOM_[A-Z0-9]{16}`, "[REDACTED:CUSTOM]")
	if err != nil {
		t.Fatalf("failed to add custom pattern: %v", err)
	}

	scrubbed, count := filter.Scrub("token: CUSTOM_ABCDEF0123456789")
	if count == 0 {
		t.Error("custom pattern not detected")
	}
	if strings.Contains(scrubbed, "CUSTOM_") {
		t.Error("custom token not redacted")
	}
}
