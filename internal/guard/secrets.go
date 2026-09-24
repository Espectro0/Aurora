package guard

import (
	"regexp"
	"sort"
	"strings"
)

type secretPattern struct {
	name string
	re   *regexp.Regexp
}

var secretPatterns = []secretPattern{
	{"openai/openrouter", regexp.MustCompile(`\bsk-(?:or-|proj-|ant-)?[A-Za-z0-9_-]{20,}`)},
	{"github", regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr|github_pat)_[A-Za-z0-9_]{20,}`)},
	{"aws", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"google", regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`)},
	{"slack", regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`)},
	{"discord", regexp.MustCompile(`\b[MN][A-Za-z\d_-]{23,25}\.[A-Za-z\d_-]{6}\.[A-Za-z\d_-]{27,}`)},
	{"telegram", regexp.MustCompile(`\b\d{8,10}:[A-Za-z0-9_-]{35}\b`)},
	{"jwt", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`)},
	{"private key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?(?:-----END [A-Z ]*PRIVATE KEY-----|$)`)},
	{"bearer", regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{16,}`)},
	{"url credentials", regexp.MustCompile(`\b[a-z][a-z0-9+.-]*://[^\s:/@]+:[^\s@/]+@`)},
	{"clave", regexp.MustCompile(`(?i)\b(?:contraseña|contrasena|password|passwd|pwd|clave|pin|token|api[_ -]?key|secret|secreto)\b\s*(?:es|is|=|:)?\s*["']?[^\s"',;]{4,}`)},
}

var cardCandidate = regexp.MustCompile(`\b(?:\d[ -]?){12,18}\d\b`)

func RedactSecrets(s string) (string, bool) {
	type span struct {
		start, end int
		kind       string
	}
	var spans []span
	for _, p := range secretPatterns {
		for _, m := range p.re.FindAllStringIndex(s, -1) {
			spans = append(spans, span{m[0], m[1], p.name})
		}
	}
	for _, m := range cardCandidate.FindAllStringIndex(s, -1) {
		digits := strings.NewReplacer(" ", "", "-", "").Replace(s[m[0]:m[1]])
		if luhn(digits) {
			spans = append(spans, span{m[0], m[1], "tarjeta"})
		}
	}
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
