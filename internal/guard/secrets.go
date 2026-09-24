package guard

import (
	"regexp"
	"sort"
	"strings"
)

// secretPattern is one detector. The regexp only runs when at least one of
// the hints (lowercase ASCII substrings, matched case-insensitively) appears
// in the input, or, for patterns with no literal to look for, when pre
// accepts it, so text without any candidate skips the regexp engine.
type secretPattern struct {
	name  string
	hints []string
	re    *regexp.Regexp
	pre   func(string) bool
}

var credentialHints = []string{"contrase", "password", "passwd", "passphrase", "pwd", "clave", "llave", "pin", "token", "api", "secret", "private", "access"}

var secretPatterns = []secretPattern{
	{"private key", []string{"private key"}, regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY(?: BLOCK)?-----[\s\S]*?(?:-----END [A-Z0-9 ]*PRIVATE KEY(?: BLOCK)?-----|$)`), nil},
	{"openai/anthropic", []string{"sk-"}, regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}`), nil},
	{"github", []string{"ghp_", "gho_", "ghu_", "ghs_", "ghr_", "github_pat_"}, regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr|github_pat)_[A-Za-z0-9_]{20,}`), nil},
	{"gitlab", []string{"glpat-"}, regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20,}`), nil},
	{"aws", []string{"akia", "asia", "abia", "acca"}, regexp.MustCompile(`\b(?:AKIA|ASIA|ABIA|ACCA)[0-9A-Z]{16}\b`), nil},
	{"google", []string{"aiza"}, regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}`), nil},
	{"google oauth", []string{"ya29."}, regexp.MustCompile(`\bya29\.[0-9A-Za-z_-]{20,}`), nil},
	{"stripe", []string{"_live_", "_test_", "whsec_"}, regexp.MustCompile(`\b(?:(?:sk|rk)_(?:live|test)_[A-Za-z0-9]{16,}|whsec_[A-Za-z0-9]{24,})`), nil},
	{"slack", []string{"xox"}, regexp.MustCompile(`\bxox[abeoprs]-[A-Za-z0-9-]{10,}`), nil},
	{"slack webhook", []string{"hooks.slack.com"}, regexp.MustCompile(`https://hooks\.slack\.com/(?:services|workflows|triggers)/[A-Za-z0-9/_-]+`), nil},
	{"discord", nil, regexp.MustCompile(`\b[MNO][A-Za-z\d_-]{23,25}\.[A-Za-z\d_-]{6}\.[A-Za-z\d_-]{27,}`), func(s string) bool { return hasTokenRun(s, 24) }},
	{"discord webhook", []string{"discord"}, regexp.MustCompile(`https://(?:[a-z]+\.)?discord(?:app)?\.com/api/webhooks/\d+/[A-Za-z0-9_-]+`), nil},
	{"telegram", []string{":"}, regexp.MustCompile(`\b\d{8,10}:[A-Za-z0-9_-]{35}`), nil},
	{"huggingface", []string{"hf_"}, regexp.MustCompile(`\bhf_[A-Za-z0-9]{30,}`), nil},
	{"npm", []string{"npm_"}, regexp.MustCompile(`\bnpm_[A-Za-z0-9]{36}`), nil},
	{"sendgrid", []string{"sg."}, regexp.MustCompile(`\bSG\.[A-Za-z0-9_-]{22}\.[A-Za-z0-9_-]{43}`), nil},
	{"jwt", []string{"eyj"}, regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`), nil},
	{"bearer", []string{"bearer"}, regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{16,}`), nil},
	{"basic auth", []string{"basic"}, regexp.MustCompile(`(?i)\bauthorization["']?\s*[:=]\s*["']?basic\s+[A-Za-z0-9+/=]{8,}`), nil},
	{"url credentials", []string{"://"}, regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^\s:/@]+:[^\s@/]+@`), nil},
	// NAME=value / "name": "value" where NAME is a compound identifier such as
	// AWS_SECRET_ACCESS_KEY or db.password. The keyword must be followed by a
	// separator or the end of the name, so max_tokens or tokenizer don't match.
	{"clave", credentialHints, regexp.MustCompile(`(?i)\b[A-Za-z0-9_.-]*(?:password|passwd|passphrase|secret|token|api_?key|private_?key|access_?key)(?:[_.-][A-Za-z0-9_.-]*)?["']?\s*[:=]\s*["']?[^\s"',;]{8,}`), nil},
	// Natural language, in Spanish or English: "mi contraseña es X", "token: X".
	// ñ is not an ASCII word character, so contraseña can't be followed by \b.
	{"clave", credentialHints, regexp.MustCompile(`(?i)\b(?:contraseña|(?:contrasena|password|passwd|passphrase|pwd|clave|llave|pin|token|api[_ -]?key|private[_ -]?key|secret|secreto)\b)["']?\s*(?:es|is|=|:)?\s*["']?[^\s"',;]{4,}`), nil},
}

var cardCandidate = regexp.MustCompile(`\b(?:\d[ -]?){12,18}\d\b`)

// hintsByFirst indexes every hint by its first byte so presentHints finds
// all of them in one pass; patternHints[i] is the set of hint bits of
// secretPatterns[i].
var (
	hintsByFirst [256][]hintRef
	patternHints []uint64
)

type hintRef struct {
	s   string
	bit uint64
}

func init() {
	ids := map[string]uint64{}
	patternHints = make([]uint64, len(secretPatterns))
	for i, p := range secretPatterns {
		for _, h := range p.hints {
			bit, ok := ids[h]
			if !ok {
				if len(ids) == 64 {
					panic("guard: more than 64 distinct secret hints")
				}
				bit = 1 << len(ids)
				ids[h] = bit
				hintsByFirst[h[0]] = append(hintsByFirst[h[0]], hintRef{h, bit})
			}
			patternHints[i] |= bit
		}
	}
}

// presentHints returns the bits of the hints that occur in s, ignoring
// ASCII case.
func presentHints(s string) uint64 {
	var found uint64
	for i := 0; i < len(s); i++ {
		for _, h := range hintsByFirst[toLowerASCII(s[i])] {
			if found&h.bit == 0 && hasPrefixFold(s[i:], h.s) {
				found |= h.bit
			}
		}
	}
	return found
}

func hasPrefixFold(s, lowerPrefix string) bool {
	if len(s) < len(lowerPrefix) {
		return false
	}
	for i := 0; i < len(lowerPrefix); i++ {
		if toLowerASCII(s[i]) != lowerPrefix[i] {
			return false
		}
	}
	return true
}

func toLowerASCII(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

// minSecretLen is the length of the shortest thing any detector matches
// ("pin 1234"); shorter strings are returned untouched without scanning.
const minSecretLen = 8

type secretSpan struct {
	start, end int
	kind       string
}

// scanSecrets calls fn for every secret found in s, stopping early when fn
// returns false. Spans may overlap and are not sorted.
func scanSecrets(s string, fn func(secretSpan) bool) {
	if len(s) < minSecretLen {
		return
	}
	hints := presentHints(s)
	for i, p := range secretPatterns {
		if p.pre != nil && !p.pre(s) || p.hints != nil && hints&patternHints[i] == 0 {
			continue
		}
		for _, m := range p.re.FindAllStringIndex(s, -1) {
			if !fn(secretSpan{m[0], m[1], p.name}) {
				return
			}
		}
	}
	if !hasDigitRun(s, 13) {
		return
	}
	for _, m := range cardCandidate.FindAllStringIndex(s, -1) {
		if isCardNumber(s[m[0]:m[1]]) && !fn(secretSpan{m[0], m[1], "tarjeta"}) {
			return
		}
	}
}

// ContainsSecret reports whether s holds anything RedactSecrets would hide.
// It stops at the first finding, so it is cheaper than RedactSecrets.
func ContainsSecret(s string) bool {
	found := false
	scanSecrets(s, func(secretSpan) bool {
		found = true
		return false
	})
	return found
}

// RedactSecrets replaces every secret in s with [OCULTO:kind] and reports
// whether anything was replaced.
func RedactSecrets(s string) (string, bool) {
	var spans []secretSpan
	scanSecrets(s, func(sp secretSpan) bool {
		spans = append(spans, sp)
		return true
	})
	if len(spans) == 0 {
		return s, false
	}

	sort.Slice(spans, func(i, j int) bool {
		if spans[i].start != spans[j].start {
			return spans[i].start < spans[j].start
		}
		return spans[i].end > spans[j].end
	})
	merged := spans[:1]
	for _, sp := range spans[1:] {
		last := &merged[len(merged)-1]
		if sp.start < last.end {
			last.end = max(last.end, sp.end)
			continue
		}
		merged = append(merged, sp)
	}

	var b strings.Builder
	b.Grow(len(s))
	prev := 0
	for _, sp := range merged {
		b.WriteString(s[prev:sp.start])
		b.WriteString("[OCULTO:")
		b.WriteString(sp.kind)
		b.WriteByte(']')
		prev = sp.end
	}
	b.WriteString(s[prev:])
	return b.String(), true
}

// hasTokenRun reports whether s has n consecutive [A-Za-z0-9_-] bytes.
func hasTokenRun(s string, n int) bool {
	run := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' {
			if run++; run >= n {
				return true
			}
		} else {
			run = 0
		}
	}
	return false
}

// hasDigitRun reports whether s has n digits in a row, allowing a single
// space or dash between them, the shape cardCandidate looks for.
func hasDigitRun(s string, n int) bool {
	run := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			if run++; run >= n {
				return true
			}
		case (c == ' ' || c == '-') && i > 0 && s[i-1] >= '0' && s[i-1] <= '9':
		default:
			run = 0
		}
	}
	return false
}

// isCardNumber checks a cardCandidate match: 13 to 19 digits, a first digit
// used by the major networks (2-6) and a valid Luhn checksum.
func isCardNumber(m string) bool {
	var buf [19]byte
	d := buf[:0]
	for i := 0; i < len(m); i++ {
		if c := m[i]; c >= '0' && c <= '9' {
			d = append(d, c)
		}
	}
	return len(d) >= 13 && d[0] >= '2' && d[0] <= '6' && luhn(string(d))
}

func luhn(d string) bool {
	sum, double := 0, false
	for i := len(d) - 1; i >= 0; i-- {
		n := int(d[i] - '0')
		if double {
			if n *= 2; n > 9 {
				n -= 9
			}
		}
		sum += n
		double = !double
	}
	return len(d) >= 13 && sum%10 == 0
}
