// Package redact removes sensitive data from job payloads before they are
// surfaced to operators (the dashboard) or written to logs.
//
// Hades jobs carry free-form Metadata maps (both job-level and per-step) that
// are injected verbatim into containers as environment variables. In practice
// these hold credentials - git tokens, registry passwords, connection strings -
// so their values must never leave the process unmasked.
//
// Two strategies are provided:
//
//   - Drop: remove every metadata entry entirely (keys and values). This mirrors
//     the historical logging behaviour and is the safest option.
//   - Redactor: keep metadata keys but mask sensitive values. A value is masked
//     when its key matches a denylist of sensitive name patterns, or when the
//     value itself looks like a secret (credentials embedded in a URL, a PEM
//     block, a JWT, or a high-entropy blob). In ModeAll every value is masked
//     regardless. Keeping keys lets operators see which variables a job defines
//     without exposing the secret values.
package redact

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/ls1intum/hades/shared/payload"
)

// Mask is the placeholder shown in place of a redacted value.
const Mask = "••••••"

// Mode selects how aggressively values are masked.
type Mode string

const (
	// ModeSmart masks values whose key matches the sensitive-key denylist or
	// whose value matches a secret heuristic. This is the default.
	ModeSmart Mode = "smart"
	// ModeAll masks every metadata value regardless of key or content.
	ModeAll Mode = "all"
)

// DefaultKeyPattern matches metadata keys that conventionally hold secrets.
// It is case-insensitive and matches the pattern anywhere in the key.
const DefaultKeyPattern = `(?i)(token|passwd|password|pwd|secret|api[_-]?key|access[_-]?key|credential|auth|private|cert|signing|session)`

// Config configures a Redactor. The zero value is not valid; use New or Default.
type Config struct {
	// Mode selects the masking strategy. Defaults to ModeSmart when empty.
	Mode Mode `env:"SECRET_REDACT_MODE" envDefault:"smart"`
	// KeyPattern is a case-insensitive regular expression; metadata keys that
	// match are masked. Defaults to DefaultKeyPattern when empty.
	KeyPattern string `env:"SECRET_KEY_PATTERNS"`
}

// Redactor masks sensitive metadata values according to its configuration.
// A Redactor is safe for concurrent use.
type Redactor struct {
	mode  Mode
	keyRe *regexp.Regexp
}

var (
	// credentialsInURL matches a scheme://user:password@host style URL.
	credentialsInURL = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.\-]*://[^/\s:@]+:[^/\s@]+@`)
	// jwtLike matches a three-segment base64url token (header.payload.signature).
	jwtLike = regexp.MustCompile(`^[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}$`)
	// tokenCharset matches strings made only of base64/hex/token characters.
	tokenCharset = regexp.MustCompile(`^[A-Za-z0-9+/=_\-]+$`)
)

// New builds a Redactor from cfg, compiling the key pattern. It returns an
// error if the key pattern is not a valid regular expression.
func New(cfg Config) (*Redactor, error) {
	mode := cfg.Mode
	if mode == "" {
		mode = ModeSmart
	}
	pattern := cfg.KeyPattern
	if pattern == "" {
		pattern = DefaultKeyPattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("compiling secret key pattern %q: %w", pattern, err)
	}
	return &Redactor{mode: mode, keyRe: re}, nil
}

// Default returns a Redactor with the built-in smart configuration.
func Default() *Redactor {
	r, err := New(Config{Mode: ModeSmart})
	if err != nil {
		// DefaultKeyPattern is a compile-time constant known to be valid.
		panic(fmt.Sprintf("redact: default config failed to compile: %v", err))
	}
	return r
}

// Value reports the display value for a single metadata entry and whether it
// was masked. Empty values are left untouched (nothing to hide).
func (r *Redactor) Value(key, value string) (string, bool) {
	if value == "" {
		return value, false
	}
	if r.mode == ModeAll || r.keyRe.MatchString(key) || looksLikeSecret(value) {
		return Mask, true
	}
	return value, false
}

// Metadata returns a redacted copy of m. The input map is never modified.
func (r *Redactor) Metadata(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		masked, _ := r.Value(k, v)
		out[k] = masked
	}
	return out
}

// Payload returns a deep copy of job with all job-level and step-level metadata
// redacted. The input payload and its slices/maps are never modified.
func (r *Redactor) Payload(job payload.QueuePayload) payload.QueuePayload {
	job.Metadata = r.Metadata(job.Metadata)
	steps := make([]payload.Step, len(job.Steps))
	copy(steps, job.Steps)
	for i := range steps {
		steps[i].Metadata = r.Metadata(steps[i].Metadata)
	}
	job.Steps = steps
	return job
}

// Drop returns a deep copy of job with every metadata map emptied (keys and
// values removed). Use this for log output where key names add no value.
func Drop(job payload.QueuePayload) payload.QueuePayload {
	job.Metadata = map[string]string{}
	steps := make([]payload.Step, len(job.Steps))
	copy(steps, job.Steps)
	for i := range steps {
		steps[i].Metadata = map[string]string{}
	}
	job.Steps = steps
	return job
}

// looksLikeSecret applies content heuristics that catch secrets regardless of
// their key name: credentials embedded in a URL, PEM key blocks, JWTs, and
// long high-entropy tokens.
func looksLikeSecret(value string) bool {
	if credentialsInURL.MatchString(value) {
		return true
	}
	if strings.Contains(value, "-----BEGIN") {
		return true
	}
	if jwtLike.MatchString(value) {
		return true
	}
	// Long, unbroken, high-entropy token strings (e.g. API keys) with no
	// whitespace are almost always secrets.
	if len(value) >= 24 && tokenCharset.MatchString(value) && shannonEntropy(value) >= 3.5 {
		return true
	}
	return false
}

// shannonEntropy returns the Shannon entropy (bits per character) of s.
func shannonEntropy(s string) float64 {
	if s == "" {
		return 0
	}
	var counts [256]float64
	for i := 0; i < len(s); i++ {
		counts[s[i]]++
	}
	n := float64(len(s))
	var entropy float64
	for _, c := range counts {
		if c == 0 {
			continue
		}
		p := c / n
		entropy -= p * math.Log2(p)
	}
	return entropy
}
