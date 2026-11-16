package web

import (
	"encoding/json"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
)

// vitalisGreen is the Stitch primary (#006947). The Cais scaffold icons are
// navy + grey; a corner pixel that is not this green means we still ship the
// placeholder.
var vitalisGreen = color.RGBA{R: 0, G: 105, B: 71, A: 255}

func TestFaviconSVG_isTheVitalisDrop(t *testing.T) {
	body, err := os.ReadFile("static/favicon.svg")
	if err != nil {
		t.Fatal(err)
	}
	svg := string(body)
	if !strings.Contains(svg, "#006947") {
		t.Error("favicon.svg is not painted in Vitalis green")
	}
	if !strings.Contains(svg, "c-4 4.5-7 8.5-7 12.5") {
		t.Error("favicon.svg is missing the in-app droplet path")
	}
}

func TestPWAIcons_areSquareVitalisGreen(t *testing.T) {
	cases := []struct {
		path string
		size int
	}{
		{"static/icons/icon-192.png", 192},
		{"static/icons/icon-512.png", 512},
		{"static/icons/icon-512-maskable.png", 512},
		{"static/icons/icon.png", 180},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			f, err := os.Open(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = f.Close() }()
			img, err := png.Decode(f)
			if err != nil {
				t.Fatal(err)
			}
			b := img.Bounds()
			if b.Dx() != tc.size || b.Dy() != tc.size {
				t.Fatalf("size = %dx%d, want %dx%d", b.Dx(), b.Dy(), tc.size, tc.size)
			}
			got := color.RGBAModel.Convert(img.At(b.Min.X, b.Min.Y)).(color.RGBA)
			if got != vitalisGreen {
				t.Errorf("corner pixel = %v, want Vitalis green %v", got, vitalisGreen)
			}
		})
	}
}

func TestManifest_isVitalisNotCaisScaffold(t *testing.T) {
	body, err := os.ReadFile("static/manifest.webmanifest")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Name            string `json:"name"`
		ShortName       string `json:"short_name"`
		ThemeColor      string `json:"theme_color"`
		BackgroundColor string `json:"background_color"`
		Description     string `json:"description"`
	}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	if m.Name != "Vitalis" {
		t.Errorf("name = %q, want Vitalis", m.Name)
	}
	if m.ShortName != "Vitalis" {
		t.Errorf("short_name = %q, want Vitalis", m.ShortName)
	}
	if m.ThemeColor != "#006947" {
		t.Errorf("theme_color = %q, want #006947", m.ThemeColor)
	}
	if m.BackgroundColor != "#f9f9ff" {
		t.Errorf("background_color = %q, want #f9f9ff", m.BackgroundColor)
	}
	if strings.Contains(strings.ToLower(m.Description), "cais") {
		t.Errorf("description still mentions Cais: %q", m.Description)
	}
}
