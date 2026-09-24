package guard

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Espectro0/AuroraProject/internal/decision"
)

const (
	RiskRead        = "lectura"
	RiskWrite       = "escritura"
	RiskDestructive = "destructiva"
	RiskThirdParty  = "terceros"
	RiskMoney       = "dinero"
	RiskSystem      = "sistema"
	RiskCredentials = "credenciales"
)

var riskCriteria = map[string]string{
	RiskRead:        "solo consulta información, no cambia nada",
	RiskWrite:       "crea o modifica algo propio del usuario (eventos, notas, archivos)",
	RiskDestructive: "borra o sobrescribe datos, o es difícil de deshacer",
	RiskThirdParty:  "envía mensajes, correos o datos a otras personas o servicios externos",
	RiskMoney:       "pagos, compras, transferencias o cualquier movimiento de dinero",
	RiskSystem:      "ejecuta comandos, instala o reinicia servicios, o cambia la configuración de un equipo",
	RiskCredentials: "crea, cambia o comparte contraseñas, llaves, tokens o permisos de acceso",
}

type JevPolicy struct {
	Judge decision.Provider

	RequestedThreshold float64 // (default 0.5)
	SecretThreshold    float64 // (default 0.6)
	Timeout            time.Duration
	IncludeNative      bool
}

func NewJevPolicy(j decision.Provider, includeNative bool) *JevPolicy {
	return &JevPolicy{Judge: j, RequestedThreshold: 0.5, SecretThreshold: 0.6, Timeout: 10 * time.Second, IncludeNative: includeNative}
}

func (p *JevPolicy) Name() string { return "jev" }

func (p *JevPolicy) Check(ctx context.Context, c Call) Verdict {
	external := c.Server != ""
	if p.Judge == nil || (!external && !p.IncludeNative) {
		return Verdict{Decision: Allow}
	}

	st, localSecret := state(c)

	ctx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	answers, err := p.Judge.Judge(ctx, st, map[string]decision.Question{
		"requested": {
			Type:         decision.TypeNoul,
			Instructions: "¿El usuario pidió explícita o claramente, en su mensaje, la acción que hace esta herramienta con estos argumentos?",
		},
		"risk": {
			Type:         decision.TypeChoice,
			Instructions: "¿Qué tipo de efecto tiene ejecutar esta herramienta con estos argumentos?",
			Criteria:     riskCriteria,
		},
		"secret": {
			Type:         decision.TypeNoul,
			Instructions: "¿Los argumentos contienen información secreta o sensible: contraseñas, tokens, claves o códigos de acceso, datos bancarios o de tarjetas, documentos de identidad, datos de salud, o información privada de otras personas?",
		},
	})
	if err != nil {
		log.Printf("[guard] jev: %v", err)
		if external {
			return Verdict{Decision: Confirm, Reason: "no se pudo evaluar la seguridad de la acción", Sensitive: localSecret}
		}
		return Verdict{Decision: Allow, Sensitive: localSecret}
	}

	requested := answers["requested"].Noul
	risk := answers["risk"].Choice
	secret := answers["secret"].Noul
	if _, ok := riskCriteria[risk]; !ok {
		risk = RiskWrite
	}

	v := Verdict{
		Decision:  Allow,
		Sensitive: localSecret || secret >= p.SecretThreshold,
		Details: map[string]any{
			"requested": round(requested),
			"risk":      risk,
			"secret":    round(secret),
			"local":     localSecret,
		},
	}

	switch {
	case risk == RiskCredentials:
		v.Decision, v.Reason = Deny, "toca credenciales o permisos de acceso"
	case risk == RiskMoney:
		v.Decision, v.Reason = Confirm, "involucra dinero"
	case risk == RiskDestructive:
		v.Decision, v.Reason = Confirm, "borra o sobrescribe datos"
	case risk == RiskSystem:
		v.Decision, v.Reason = Confirm, "ejecuta cambios en un sistema"
	case v.Sensitive && (external || risk == RiskThirdParty):
		v.Decision, v.Reason = Confirm, "enviaría información sensible a un servicio externo"
	case requested < p.RequestedThreshold && risk != RiskRead:
		v.Decision, v.Reason = Confirm, "el usuario no pidió esta acción"
	}
	return v
}

func state(c Call) (string, bool) {
	var b strings.Builder
	fmt.Fprintf(&b, "Mensaje del usuario: %s\n", orNone(c.UserMessage))
	if c.Server != "" {
		fmt.Fprintf(&b, "Herramienta: %s (servidor externo MCP %q)\n", c.Tool, c.Server)
	} else {
		fmt.Fprintf(&b, "Herramienta: %s (skill propia de Aurora)\n", c.Skill)
	}
	fmt.Fprintf(&b, "Descripción: %s\n", orNone(c.Description))
	args, _ := json.Marshal(redactArgs(c.ArgsJSON))
	fmt.Fprintf(&b, "Argumentos: %s", args)

	out, _ := RedactSecrets(b.String())
	_, inArgs := RedactSecrets(c.ArgsJSON)
	return out, inArgs || argsHaveSecretKeys(c.ArgsJSON)
}

func orNone(s string) string {
	if strings.TrimSpace(s) == "" {
		return "(ninguno)"
	}
	return s
}

func round(f float64) float64 { return float64(int(f*100+0.5)) / 100 }
