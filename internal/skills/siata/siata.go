package siata

import (
	"fmt"
	"image"
	"net/http"
	"net/url"
	"path"

	sm "github.com/flopp/go-staticmaps"
	"github.com/fogleman/gg"
	"github.com/golang/geo/s2"
	"golang.org/x/image/font/basicfont"

	"github.com/Espectro0/AuroraProject/internal/httpclient"
)

const baseURL = "https://siata.gov.co/data/siata_app/"

type RadarFrame struct {
	Time  string `json:"time"`
	Image string `json:"image"`
}

type RadarResp struct {
	North float64      `json:"north"`
	South float64      `json:"south"`
	East  float64      `json:"east"`
	West  float64      `json:"west"`
	Urls  []RadarFrame `json:"urls"`
}

type RenderResult struct {
	FrameTime  string
	OutputPath string
}

type Client struct {
	http *httpclient.Client
}

func NewClient(http *httpclient.Client) *Client {
	return &Client{http: http}
}

func (c *Client) RadarImage() (RadarResp, error) {
	var resp RadarResp
	if err := c.http.FetchJSON(baseURL+"animacion_radar.json", &resp); err != nil {
		return RadarResp{}, fmt.Errorf("siata: obteniendo metadata del radar: %w", err)
	}
	return resp, nil
}

func lastFrame(resp RadarResp) (RadarFrame, error) {
	if len(resp.Urls) == 0 {
		return RadarFrame{}, fmt.Errorf("siata: sin imagenes disponibles")
	}

	raw := resp.Urls[len(resp.Urls)-1].Image
	u, err := url.Parse(raw)
	if err != nil {
		return RadarFrame{}, err
	}
	u.Path = path.Clean(u.Path)

	return RadarFrame{
		Time:  resp.Urls[len(resp.Urls)-1].Time,
		Image: u.String(),
	}, nil
}

func (c *Client) fetchImage(rawURL string) (image.Image, error) {
	var img image.Image
	err := c.http.Fetch(rawURL, func(resp *http.Response) error {
		decoded, _, err := image.Decode(resp.Body)
		if err != nil {
			return fmt.Errorf("decodificando imagen: %w", err)
		}
		img = decoded
		return nil
	})
	if err != nil {
		return nil, err
	}
	return img, nil
}

type radarOverlay struct {
	Img   image.Image
	North float64
	South float64
	East  float64
	West  float64
}

func (r *radarOverlay) Bounds() s2.Rect {
	rect := s2.EmptyRect()
	rect = rect.AddPoint(s2.LatLngFromDegrees(r.North, r.West))
	rect = rect.AddPoint(s2.LatLngFromDegrees(r.South, r.East))
	return rect
}

func (r *radarOverlay) ExtraMarginPixels() (float64, float64, float64, float64) {
	return 0, 0, 0, 0
}

func (r *radarOverlay) Draw(dc *gg.Context, trans *sm.Transformer) {
	xNW, yNW := trans.LatLngToXY(s2.LatLngFromDegrees(r.North, r.West))
	xSE, ySE := trans.LatLngToXY(s2.LatLngFromDegrees(r.South, r.East))

	w := xSE - xNW
	h := ySE - yNW

	dc.Push()
	dc.Translate(xNW, yNW)
	dc.Scale(w/float64(r.Img.Bounds().Dx()), h/float64(r.Img.Bounds().Dy()))
	dc.DrawImage(r.Img, 0, 0)
	dc.Pop()
}

func (c *Client) RenderMap(outputPath string, zoom int) (RenderResult, error) {
	meta, err := c.RadarImage()
	if err != nil {
		return RenderResult{}, err
	}

	frame, err := lastFrame(meta)
	if err != nil {
		return RenderResult{}, fmt.Errorf("siata: resolviendo ultimo frame: %w", err)
	}

	radarImg, err := c.fetchImage(frame.Image)
	if err != nil {
		return RenderResult{}, fmt.Errorf("siata: descargando frame del radar: %w", err)
	}

	bbox, err := sm.CreateBBox(meta.North, meta.West, meta.South, meta.East)
	if err != nil {
		return RenderResult{}, fmt.Errorf("siata: creando bbox: %w", err)
	}

	ctx := sm.NewContext()
	ctx.SetSize(1600, 1200)
	ctx.SetBoundingBox(*bbox)
	ctx.SetZoom(zoom)
	ctx.AddObject(&radarOverlay{
		Img:   radarImg,
		North: meta.North,
		South: meta.South,
		East:  meta.East,
		West:  meta.West,
	})

	mapImg, err := ctx.Render()
	if err != nil {
		return RenderResult{}, fmt.Errorf("siata: renderizando mapa: %w", err)
	}

	dc := gg.NewContextForImage(mapImg)
	label := fmt.Sprintf("%s\n%s", baseURL, frame.Time)
	dc.SetFontFace(basicfont.Face7x13)
	const pad = 8.0
	tw, th := dc.MeasureMultilineString(label, 1.4)
	dc.SetRGBA(1, 1, 1, 0.7)
	dc.DrawRectangle(0, 0, tw+2*pad, th+2*pad)
	dc.Fill()
	dc.SetRGB(0, 0, 0)
	dc.DrawStringWrapped(label, pad, pad, 0, 0, tw, 1.4, gg.AlignLeft)
	mapImg = dc.Image()

	if err := gg.SavePNG(outputPath, mapImg); err != nil {
		return RenderResult{}, fmt.Errorf("siata: guardando png: %w", err)
	}

	return RenderResult{FrameTime: frame.Time, OutputPath: outputPath}, nil
}
