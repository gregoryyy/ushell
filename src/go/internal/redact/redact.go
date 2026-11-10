// Package redact removes likely secrets from text before it leaves the machine or is stored
// (ushpy redact.py). Redaction applies to command output sent to the AI provider, to piped input,
// and to answers saved in history; what the user sees in the terminal is not redacted. Pattern
// matching is best effort: it reduces, but cannot rule out, disclosure.
//
// Go's regexp (RE2) has no lookaround, so where ushpy's patterns look behind or ahead, the
// context is captured and written back. RE2 matches in linear time, so no input can make
// redaction slow (SECTEST F3).
package redact

import (
	"regexp"
	"strings"
)

// Mark replaces each redacted value.
const Mark = "[redacted]"

// whole are replaced entirely.
var whole = []*regexp.Regexp{
	// Private key blocks
	regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY( BLOCK)?-----.*?(-----END [A-Z0-9 ]*PRIVATE KEY( BLOCK)?-----|\z)`),
	// Provider and platform tokens
	regexp.MustCompile(`\bsk-(?:ant-)?[A-Za-z0-9_\-]{20,}`),
	regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`),
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{30,}\b`),
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{40,}\b`),
	regexp.MustCompile(`\bxox[abposr]-[A-Za-z0-9\-]{10,}`),
	regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{35}\b`),
	regexp.MustCompile(`\bglpat-[A-Za-z0-9_\-]{20,}\b`),
	regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{10,}\.eyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}`),
	regexp.MustCompile(`\b[rs]k_(?:live|test)_[A-Za-z0-9]{16,}`),         // Stripe
	regexp.MustCompile(`\bya29\.[A-Za-z0-9_\-]{20,}`),                    // Google OAuth
	regexp.MustCompile(`\bnpm_[A-Za-z0-9]{36}\b`),                        // npm
	regexp.MustCompile(`\bpypi-AgE[A-Za-z0-9_\-]{40,}`),                  // PyPI
	regexp.MustCompile(`\bSG\.[A-Za-z0-9_\-]{16,}\.[A-Za-z0-9_\-]{16,}`), // SendGrid
	regexp.MustCompile(`\bhooks\.slack\.com/services/[A-Za-z0-9/]+`),     // Slack webhook
	regexp.MustCompile(`\bdop_v1_[a-f0-9]{64}\b`),                        // DigitalOcean
	regexp.MustCompile(`\bhf_[A-Za-z0-9]{30,}\b`),                        // Hugging Face
}

// kept: group 1 (and group 3, if any) are kept around the redacted group 2.
var kept = []*regexp.Regexp{
	// HTTP authorization values: Bearer and Basic credentials
	regexp.MustCompile(`(?i)(\bbearer )([A-Za-z0-9._~+/\-]{12,}=*)()`),
	regexp.MustCompile(`(?i)(\bbasic )([A-Za-z0-9+/]{12,}=*)()`),
	// .netrc and similar: "password VALUE"
	regexp.MustCompile(`(?i)(\bpassword )(\S+)()`),
	// Credentials embedded in URLs: scheme://user:password@host
	regexp.MustCompile(`(://)([^/\s:@]+:[^/\s@]+)(@)`),
}

// assignment: NAME=value, NAME: value, "name": "value" for secret-like names; the name is kept.
// A name starts a word (group 1 is what precedes it), as in ushpy.
var assignment = regexp.MustCompile(`(?i)(^|[^\w.\-"'])` +
	`(["']?[\w.\-]{0,40}(?:password|passwd|passphrase|secret|token|api[_\-]?key|access[_\-]?key|` +
	`private[_\-]?key|account[_\-]?key|auth|credential|session[_\-]?id)[\w.\-]{0,40}["']?)` +
	`(\s*[:=]\s*)("[^"\n]*"|'[^'\n]*'|[^\s,;}\]]+)`)

// Text returns text with likely secrets, and each of known (8 characters or more), replaced.
func Text(text string, known ...string) string {
	for _, secret := range known {
		if len(secret) >= 8 {
			text = strings.ReplaceAll(text, secret, Mark)
		}
	}
	for _, re := range whole {
		text = re.ReplaceAllLiteralString(text, Mark)
	}
	for _, re := range kept {
		text = re.ReplaceAllStringFunc(text, func(m string) string {
			g := re.FindStringSubmatch(m)
			if g[2] == Mark {
				return m
			}
			return g[1] + Mark + g[3]
		})
	}
	return assignment.ReplaceAllStringFunc(text, func(m string) string {
		g := assignment.FindStringSubmatch(m)
		value := strings.Trim(g[4], `"'`)
		switch strings.ToLower(value) {
		case "", Mark, "true", "false", "null", "none", "yes", "no":
			return m
		}
		return g[1] + g[2] + g[3] + Mark
	})
}
