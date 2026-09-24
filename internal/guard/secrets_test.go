package guard

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

// fake builds a token-shaped string at runtime so no literal credential
// lives in the source.
func fake(prefix string, n int) string {
	const alphabet = "aB3dE5gH7jK9mN1pQ"
	var b strings.Builder
	b.WriteString(prefix)
	for i := 0; i < n; i++ {
		b.WriteByte(alphabet[i%len(alphabet)])
	}
	return b.String()
}

func TestRedactSecretsDetects(t *testing.T) {
	cases := []struct {
		name, in, secret, kind string
	}{
		{"openai", "key " + fake("sk-proj-", 40), fake("sk-proj-", 40), "openai/anthropic"},
		{"anthropic", "key " + fake("sk-ant-api03-", 40), fake("sk-ant-api03-", 40), "openai/anthropic"},
		{"github", "tok " + fake("ghp_", 36), fake("ghp_", 36), "github"},
		{"github pat", fake("github_pat_", 60), fake("github_pat_", 60), "github"},
		{"gitlab", "x " + fake("glpat-", 20), fake("glpat-", 20), "gitlab"},
		{"aws", "id AKIA" + strings.Repeat("Q7", 8) + " ok", "AKIA" + strings.Repeat("Q7", 8), "aws"},
		{"aws temp", "id ASIA" + strings.Repeat("Z2", 8), "ASIA" + strings.Repeat("Z2", 8), "aws"},
		{"google", "k " + fake("AIza", 35), fake("AIza", 35), "google"},
		{"google ending in dash", "k AIza" + strings.Repeat("a", 34) + "- end", "AIza" + strings.Repeat("a", 34) + "-", "google"},
		{"google oauth", fake("ya29.", 40), fake("ya29.", 40), "google oauth"},
		{"stripe", "sk_" + fake("live_", 24), "sk_" + fake("live_", 24), "stripe"},
		{"slack", fake("xoxb-", 30), fake("xoxb-", 30), "slack"},
		{"slack webhook", "post to https://hooks.slack.com/services/" + fake("T", 10) + "/" + fake("B", 10), "https://hooks.slack.com/services/" + fake("T", 10) + "/" + fake("B", 10), "slack webhook"},
		{"discord webhook", "https://discord.com/api/webhooks/123456789/" + fake("", 60), "https://discord.com/api/webhooks/123456789/" + fake("", 60), "discord webhook"},
		{"telegram", "bot 1234567890:" + fake("", 35), "1234567890:" + fake("", 35), "telegram"},
		{"huggingface", fake("hf_", 34), fake("hf_", 34), "huggingface"},
		{"npm", fake("npm_", 36), fake("npm_", 36), "npm"},
		{"jwt", fake("eyJ", 20) + "." + fake("", 20) + "." + fake("", 20), fake("eyJ", 20) + "." + fake("", 20) + "." + fake("", 20), "jwt"},
		{"bearer", "Authorization: Bearer " + fake("", 30), "Bearer " + fake("", 30), "bearer"},
		{"basic", "Authorization: Basic " + fake("", 16) + "==", "Authorization: Basic " + fake("", 16) + "==", "basic auth"},
		{"url credentials", "postgres://admin:" + fake("", 12) + "@db:5432", "postgres://admin:" + fake("", 12) + "@", "url credentials"},
		{"private key", "-----BEGIN RSA PRIVATE KEY-----\nMIIE\n-----END RSA PRIVATE KEY-----", "-----BEGIN RSA PRIVATE KEY-----\nMIIE\n-----END RSA PRIVATE KEY-----", "private key"},
		{"card", "tarjeta 4111 1111 1111 1111 vence", "4111 1111 1111 1111", "tarjeta"},
		{"contraseña", "mi contraseña es hunter22", "contraseña es hunter22", "clave"},
		{"contraseña colon", "contraseña: hunter22", "contraseña: hunter22", "clave"},
		{"json key", `{"password":"hunter22"}`, `password":"hunter22`, "clave"},
		{"env compound", "AWS_SECRET_ACCESS_KEY=" + fake("", 40), "AWS_SECRET_ACCESS_KEY=" + fake("", 40), "clave"},
		{"dotted key", "db.password: " + fake("", 12), "db.password: " + fake("", 12), "clave"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, ok := RedactSecrets(tc.in)
			if !ok {
				t.Fatalf("not detected: %q", tc.in)
			}
			if strings.Contains(out, tc.secret) {
				t.Fatalf("secret still visible: %q", out)
			}
			if !strings.Contains(out, "[OCULTO:"+tc.kind+"]") {
				t.Fatalf("want kind %q, got %q", tc.kind, out)
			}
			if !ContainsSecret(tc.in) {
				t.Fatalf("ContainsSecret disagrees with RedactSecrets")
			}
		})
	}
}

func TestRedactSecretsIgnores(t *testing.T) {
	for _, in := range []string{
		"",
		"hola",
		"task-management-system-overview-doc",
		"max_tokens=100",
		"el pedido 1234567890123 llegó", // not Luhn-valid
		"id 1000000000000008",           // Luhn-valid, but no card network starts with 1
		"llamar al 3001234567 mañana",
		"https://example.com/path?q=1",
		"see the tokenizer_config.json file",
	} {
		if out, ok := RedactSecrets(in); ok {
			t.Errorf("false positive on %q: %q", in, out)
		}
		if ContainsSecret(in) {
			t.Errorf("ContainsSecret false positive on %q", in)
		}
	}
}

func TestRedactSecretsMergesOverlaps(t *testing.T) {
	tok := fake("ghp_", 36)
	out, ok := RedactSecrets("token=" + tok + " y " + tok)
	if !ok || strings.Contains(out, "ghp_") {
		t.Fatalf("got %q", out)
	}
	if strings.Count(out, "[OCULTO:") != 2 {
		t.Fatalf("want 2 redactions, got %q", out)
	}
}

func TestIsSecret(t *testing.T) {
	secret := []string{
		"token", "access_token", "accessToken", "AUTH_TOKEN", "password", "db.password", "pwd",
		"api_key", "apiKey", "x-api-key", "APIKey", "openai_api_key", "client_secret",
		"AWS_SECRET_ACCESS_KEY", "Authorization", "set-cookie", "credentials", "private_key", "refreshtoken",
	}
	public := []string{
		"max_tokens", "tokens", "tokenizer", "secretary", "keyboard", "query", "path", "monkey", "passenger", "",
	}
	for _, k := range secret {
		if !isSecret(k) {
			t.Errorf("isSecret(%q) = false, want true", k)
		}
	}
	for _, k := range public {
		if isSecret(k) {
			t.Errorf("isSecret(%q) = true, want false", k)
		}
	}
}

func TestRedactArgs(t *testing.T) {
	tok := fake("ghp_", 36)
	in, _ := json.Marshal(map[string]any{
		"query":      "hola",
		"max_tokens": 100,
		"api_key":    "abc",
		"nested":     []any{map[string]any{"note": "usa " + tok}},
		tok:          1,
	})
	v, found := redactArgs(string(in))
	if !found {
		t.Fatal("secrets not reported")
	}
	out, _ := json.Marshal(v)
	s := string(out)
	if strings.Contains(s, "ghp_") || strings.Contains(s, "abc") {
		t.Fatalf("secret leaked: %s", s)
	}
	for _, want := range []string{`"query":"hola"`, `"max_tokens":100`, `"api_key":"***"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}

	if _, found := redactArgs(`{"query":"hola","max_tokens":5,"token":""}`); found {
		t.Error("harmless args reported as secret")
	}
	if v, found := redactArgs(`not json password=` + fake("", 12)); !found || strings.Contains(v.(string), fake("", 12)) {
		t.Errorf("invalid JSON not redacted: %v %v", v, found)
	}
}

func TestRedactStringLong(t *testing.T) {
	tok := fake("sk-proj-", 40)
	// A secret far past the display window is still reported.
	long := strings.Repeat("ñ", redactWindow*2) + tok
	out, found := redactString(long)
	if !found {
		t.Fatal("secret beyond the window not reported")
	}
	if utf8.RuneCountInString(out) != maxArgChars+1 || !strings.HasSuffix(out, "…") || !utf8.ValidString(out) {
		t.Fatalf("bad clip: %d runes", utf8.RuneCountInString(out))
	}

	// A secret straddling the visible limit is redacted, not cut in half.
	edge := strings.Repeat("a ", (maxArgChars-10)/2) + tok + strings.Repeat(" b", redactWindow)
	out, found = redactString(edge)
	if !found || strings.Contains(out, "sk-proj") {
		t.Fatalf("secret at the edge leaked: %q", out[len(out)-40:])
	}

	// An unterminated private key hides everything after it.
	pk := "-----BEGIN PRIVATE KEY-----\n" + strings.Repeat("A", redactWindow*2)
	if out, _ = redactString(pk); strings.Contains(out, "AAAA") || !strings.HasSuffix(out, "…") {
		t.Fatalf("private key leaked: %q", out)
	}
}

func BenchmarkRedactSecrets(b *testing.B) {
	inputs := map[string]string{
		"clean":  strings.Repeat("Recuérdame comprar pan y leche mañana a las 9. ", 20),
		"secret": strings.Repeat("texto normal ", 20) + "clave=" + fake("", 20) + " y " + fake("ghp_", 36),
	}
	for name, in := range inputs {
		b.Run(name, func(b *testing.B) {
			b.SetBytes(int64(len(in)))
			for b.Loop() {
				RedactSecrets(in)
			}
		})
	}
}
