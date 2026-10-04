package siata

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Espectro0/AuroraProject/internal/skills"
)

const defaultZoom = 10

var _ skills.Skill = (*Skill)(nil)

type Skill struct {
	client *Client
}

func New(client *Client) *Skill {
	return &Skill{client: client}
}

func (s *Skill) Name() string {
	return "radar_siata"
}

func (s *Skill) Description() string {
	return "Genera y envía una imagen con el último frame disponible del radar meteorológico de SIATA. Úsala cuando el usuario pida ver el radar."
}

func (s *Skill) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (s *Skill) Execute(ctx context.Context, argsJSON string) (skills.Result, error) {
	filename := fmt.Sprintf("siata_radar_%d.png", time.Now().UnixNano())
	outputPath := filepath.Join(os.TempDir(), filename)

	result, err := s.client.RenderMap(outputPath, defaultZoom)
	if err != nil {
		return skills.Result{}, err
	}

	out, err := json.Marshal(map[string]string{
		"frame_time": result.FrameTime,
		"status":     "imagen del radar enviada al usuario",
	})
	if err != nil {
		return skills.Result{}, err
	}

	return skills.Result{
		Text: string(out),
		Attachments: []skills.Attachment{
			{Path: result.OutputPath, Filename: "radar_siata.png"},
		},
	}, nil
}
