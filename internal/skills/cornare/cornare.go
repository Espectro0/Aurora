package cornare

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Espectro0/AuroraProject/internal/httpclient"
)

const (
	baseURL   = "https://marco.cornare.gov.co/api/v1/estaciones/"
	cachePath = "internal/skills/cornare/cache/response.json"
	cacheTTL  = 1 * time.Hour
)

type Foto struct {
	ID        int    `json:"id"`
	Nombre    string `json:"nombre"`
	IsPortada bool   `json:"is_portada"`
	Foto      string `json:"foto"`
}

type Sensor struct {
	ID                   int     `json:"id"`
	Estacion             int     `json:"estacion"`
	Parametro            string  `json:"parametro"`
	ParametroCodigo      string  `json:"parametro_codigo"`
	ParametroNombreCorto string  `json:"parametro_nombre_corto"`
	Codigo               string  `json:"codigo"`
	Categoria            string  `json:"categoria"`
	CategoriaValue       string  `json:"categoria_value"`
	Valor                float64 `json:"valor"`
	Color                string  `json:"color"`
	Indice               int     `json:"indice"`
}

type Estacion struct {
	ID             string            `json:"id"`
	Codigo         string            `json:"codigo"`
	Municipio      int               `json:"municipio"`
	Region         int               `json:"region"`
	UbicacionCampo string            `json:"ubicacion_campo"`
	Latitud        float64           `json:"latitud"`
	Longitud       float64           `json:"longitud"`
	Red            string            `json:"red"`
	Clasificacion  string            `json:"clasificacion"`
	Corriente      string            `json:"corriente"`
	Label          string            `json:"label"`
	Foto           *string           `json:"foto"`
	Sensores       map[string]Sensor `json:"sensores"`
	Fotos          []Foto            `json:"fotos"`
}

type estacionesResp struct {
	Values []Estacion `json:"values"`
}

type Client struct {
	http *httpclient.Client
}

func NewClient(http *httpclient.Client) *Client {
	return &Client{http: http}
}

func (c *Client) Estaciones() ([]Estacion, error) {
	if info, err := os.Stat(cachePath); err == nil {
		if time.Since(info.ModTime()) < cacheTTL {
			estaciones, err := readCache()
			if err == nil {
				return estaciones, nil
			}
		}
	}

	var resp estacionesResp
	if err := c.http.FetchJSON(baseURL, &resp); err != nil {
		if estaciones, cacheErr := readCache(); cacheErr == nil {
			return estaciones, nil
		}
		return nil, fmt.Errorf("cornare: obteniendo estaciones: %w", err)
	}

	if err := writeCache(resp); err != nil {
		return nil, fmt.Errorf("cornare: guardando cache: %w", err)
	}

	return resp.Values, nil
}

func readCache() ([]Estacion, error) {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}
	var resp estacionesResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Values, nil
}

func writeCache(resp estacionesResp) error {
	data, err := json.MarshalIndent(resp, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(cachePath, data, 0644)
}
