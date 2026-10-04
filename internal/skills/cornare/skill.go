package cornare

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Espectro0/AuroraProject/internal/skills"
)

var _ skills.Skill = (*Skill)(nil)

type Skill struct {
	client *Client
}

func New(client *Client) *Skill {
	return &Skill{client: client}
}

func (s *Skill) Name() string {
	return "cornare"
}

func (s *Skill) Description() string {
	return "Obtiene información geoambiental y climática de estaciones de Cornare en el Oriente Antioqueño " +
		"(municipios como Rionegro, Marinilla, La Ceja, El Carmen de Viboral, El Retiro, El Santuario, Guarne, " +
		"La Unión, San Vicente Ferrer, El Peñol, Guatapé, San Rafael, San Carlos, Granada, Alejandría, " +
		"Concepción, San Roque, Santo Domingo, Abejorral, Argelia, Nariño, Sonsón, San Francisco, San Luis y " +
		"Puerto Triunfo). Úsala cuando el usuario pida datos de nivel de ríos/quebradas, caudal, precipitación " +
		"u otras variables ambientales medidas por Cornare en alguno de estos municipios o estaciones."
}

func (s *Skill) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"municipio": map[string]any{
				"type":        "string",
				"description": "Nombre del municipio del Oriente Antioqueño para filtrar las estaciones (ej. 'Rionegro', 'La Ceja'). Opcional.",
			},
			"parametro": map[string]any{
				"type":        "string",
				"description": "Código del parámetro a filtrar (ej. 'nivel', 'precipitacion'). Opcional.",
			},
		},
	}
}

type executeArgs struct {
	Municipio string `json:"municipio"`
	Parametro string `json:"parametro"`
}

type stationSummary struct {
	Codigo    string             `json:"codigo"`
	Municipio string             `json:"municipio"`
	Region    string             `json:"region"`
	Ubicacion string             `json:"ubicacion"`
	Corriente string             `json:"corriente"`
	Sensores  map[string]float64 `json:"sensores"`
}

func (s *Skill) Execute(ctx context.Context, argsJSON string) (skills.Result, error) {
	var args executeArgs
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return skills.Result{}, fmt.Errorf("cornare: parseando argumentos: %w", err)
		}
	}

	estaciones, err := s.client.Estaciones()
	if err != nil {
		return skills.Result{}, err
	}

	municipioFilter := strings.TrimSpace(strings.ToLower(args.Municipio))
	parametroFilter := strings.TrimSpace(strings.ToLower(args.Parametro))

	var results []stationSummary
	for _, e := range estaciones {
		info, known := stationMunicipios[e.Codigo]

		if municipioFilter != "" {
			if !known || !strings.Contains(strings.ToLower(info.Municipio), municipioFilter) {
				continue
			}
		}

		sensores := make(map[string]float64)
		for _, sensor := range e.Sensores {
			if parametroFilter != "" && !strings.Contains(strings.ToLower(sensor.ParametroCodigo), parametroFilter) {
				continue
			}
			sensores[sensor.ParametroCodigo] = sensor.Valor
		}
		if parametroFilter != "" && len(sensores) == 0 {
			continue
		}

		results = append(results, stationSummary{
			Codigo:    e.Codigo,
			Municipio: info.Municipio,
			Region:    info.Region,
			Ubicacion: e.UbicacionCampo,
			Corriente: e.Corriente,
			Sensores:  sensores,
		})
	}

	out, err := json.Marshal(map[string]any{
		"total":      len(results),
		"estaciones": results,
	})
	if err != nil {
		return skills.Result{}, err
	}

	return skills.Result{Text: string(out)}, nil
}
