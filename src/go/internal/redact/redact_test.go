package redact

import (
	"strings"
	"testing"
	"time"
)

// The corpus of ushpy's RedactionCorpusTest and test_redact; synthetic secrets are assembled at
// run time, so the repository holds no string a secret scanner would take for a real token.
func TestKnownFormatsAreRedacted(t *testing.T) {
	r := strings.Repeat
	cases := map[string][2]string{
		"anthropic":       {"sk-ant-api03-", r("A", 40)},
		"openai":          {"sk-proj-", r("B", 40)},
		"aws key id":      {"AKIA", r("C", 16)},
		"aws secret":      {"aws_secret_access_key = ", r("d", 40)},
		"github":          {"ghp_", r("e", 36)},
		"github pat":      {"github_pat_", r("f", 60)},
		"gitlab":          {"glpat-", r("g", 20)},
		"slack":           {"xoxb-1234567890-", "abcdefghij"},
		"slack webhook":   {"https://hooks.slack.com/services/T000/B000/", "XXXXXXXXXXXX"},
		"google api":      {"AIza", r("h", 35)},
		"google oauth":    {"ya29.", r("i", 40)},
		"stripe":          {"sk_live_", r("j", 24)},
		"npm":             {"npm_", r("k", 36)},
		"pypi":            {"pypi-AgEIcHlwaS5vcmc", r("l", 50)},
		"sendgrid":        {"SG." + r("m", 22) + ".", r("n", 43)},
		"digitalocean":    {"dop_v1_", r("a", 64)},
		"hugging face":    {"hf_", r("o", 34)},
		"bearer":          {"Authorization: Bearer ", "abcdefghijklmnop1234567890"},
		"basic":           {"Authorization: Basic ", "dXNlcjpwYXNzd29yZDEyMw=="},
		"netrc":           {"machine example.com login me password ", "hunter2secret"},
		"url credentials": {"https://me:", "hunter2@example.com/x"},
		"azure":           {"AccountName=x;AccountKey=", r("p", 40)},
		"env password":    {"DB_PASSWORD=", "hunter2"},
		"json token":      {`{"access_token": "`, r("q", 30) + `"}`},
		"jwt":             {"ey" + "JhbGciOiJIUzI1NiJ9.ey" + "JzdWIiOiIxMjM0NTY3ODkwIn0.", "dozjgNryP4J3jVmNHl0w5N"},
	}
	for name, c := range cases {
		secret := strings.Split(c[1], "@")[0]
		if len(secret) > 12 {
			secret = secret[:12]
		}
		if got := Text(c[0] + c[1]); strings.Contains(got, secret) {
			t.Errorf("%s: %q", name, got)
		}
	}
	for _, kind := range []string{"RSA PRIVATE KEY", "OPENSSH PRIVATE KEY", "PGP PRIVATE KEY BLOCK"} {
		block := "-----BEGIN " + kind + "-----\nMIIEsecret\n-----END " + kind + "-----"
		if got := Text(block); strings.Contains(got, "MIIEsecret") {
			t.Errorf("%s: %q", kind, got)
		}
	}
}

func TestNamesAreKeptAndPlainValuesStay(t *testing.T) {
	cases := map[string]string{
		"export GITHUB_" + "TOKEN=abc123" + `def456 and "api_key": "zzz" and x.password: hunter2`: `export GITHUB_TOKEN=[redacted] and "api_key": [redacted] and x.password: [redacted]`,
		"auth: true, token: null":                                 "auth: true, token: null",
		"disk0s2 460Gi 444Gi 16Gi 97% /System; token budget: low": "disk0s2 460Gi 444Gi 16Gi 97% /System; token budget: low",
		"password [redacted]":                                     "password [redacted]",
	}
	for in, want := range cases {
		if got := Text(in); got != want {
			t.Errorf("Text(%q)\n got %q\nwant %q", in, got, want)
		}
	}
	if got := Text("key is s3cr3t-value here", "s3cr3t-value"); got != "key is [redacted] here" {
		t.Errorf("known secret: %q", got)
	}
	if got := Text("short abc", "abc"); got != "short abc" {
		t.Errorf("short known secrets are not replaced: %q", got)
	}
}

func TestLargeAdversarialInputIsFast(t *testing.T) { // SECTEST F3
	for _, text := range []string{strings.Repeat("x", 300_000), strings.Repeat("password", 40_000),
		strings.Repeat("token_", 50_000) + "=v", strings.Repeat("a=", 150_000), strings.Repeat(".", 300_000)} {
		start := time.Now()
		Text(text)
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("%q…: %v", text[:16], d)
		}
	}
}
