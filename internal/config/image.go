// [INPUT]: The current MyGo locale, a user-selected PNG/JPEG, or a PNG saved in config.
// [OUTPUT]: Bounded 512-pixel icons with consistent transparent margins, rounded opaque artwork, and preserved aspect/alpha; saved images decode unchanged.
// [POS]: Icon input/persistence boundary; appearance/ applies platform changes and view/ owns native previews.
// [PROTOCOL]: Update this header, internal/config/AGENTS.md, and settings notes.
package config

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"io"
	"os"
	"runtime"

	l10n "github.com/ohxxx/PageDock/internal/locale"
	"golang.org/x/image/draw"
	"golang.org/x/image/vector"
)

const (
	IconSupported   = runtime.GOOS == "darwin" || runtime.GOOS == "windows"
	iconSize        = 512
	iconMargin      = iconSize / 10
	iconCornerRatio = 0.22
	// Cubic Bezier control distance for a quarter circle, relative to its radius.
	iconCurveControl = 0.55228475
	MaxIconBytes     = 8 << 20
	maxIconDimension = 4096
)

func ReadIcon(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf(l10n.Text("iconRead"), err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New(l10n.Text("iconRegular"))
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxIconBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > MaxIconBytes {
		return "", errors.New(l10n.Text("iconTooLarge"))
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") {
		return "", errors.New(l10n.Text("iconFormat"))
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxIconDimension || cfg.Height > maxIconDimension {
		return "", errors.New(l10n.Text("iconDimensions"))
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf(l10n.Text("iconDamaged"), err)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, normalizeIcon(src)); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(out.Bytes()), nil
}

func DecodeIcon(encoded string) ([]byte, error) {
	if encoded == "" {
		return nil, nil
	}
	if len(encoded) > base64.StdEncoding.EncodedLen(MaxIconBytes) {
		return nil, errors.New(l10n.Text("savedIconLarge"))
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, errors.New(l10n.Text("savedIconEncoding"))
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width != iconSize || cfg.Height != iconSize {
		return nil, errors.New(l10n.Text("savedIconInvalid"))
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		return nil, errors.New(l10n.Text("savedIconDamaged"))
	}
	return data, nil
}

func normalizeIcon(src image.Image) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, iconSize, iconSize))
	bounds, opaque := iconContentBounds(src)
	if bounds.Empty() {
		return dst
	}
	// Size the visible artwork, so existing transparent margins do not shrink it twice.
	contentSize := iconSize - 2*iconMargin
	w, h := contentSize, contentSize
	if bounds.Dx() > bounds.Dy() {
		h = max(1, contentSize*bounds.Dy()/bounds.Dx())
	} else {
		w = max(1, contentSize*bounds.Dx()/bounds.Dy())
	}
	start := image.Pt((iconSize-w)/2, (iconSize-h)/2)
	target := image.Rectangle{Min: start, Max: start.Add(image.Pt(w, h))}
	if bounds.Size() == target.Size() {
		// Preserve already-normalized edge colors without another premultiplied resample.
		draw.Draw(dst, target, src, bounds.Min, draw.Src)
	} else {
		draw.CatmullRom.Scale(dst, target, src, bounds, draw.Src, nil)
	}
	// Existing rounded corners, cutouts, shadows, and translucent shapes keep their alpha.
	if opaque {
		roundIconCorners(dst, target)
	}
	return dst
}

func iconContentBounds(src image.Image) (image.Rectangle, bool) {
	canvas := src.Bounds()
	bounds := image.Rectangle{Min: canvas.Max, Max: canvas.Min}
	opaquePixels := 0
	for y := canvas.Min.Y; y < canvas.Max.Y; y++ {
		for x := canvas.Min.X; x < canvas.Max.X; x++ {
			_, _, _, alpha := src.At(x, y).RGBA()
			if alpha == 0 {
				continue
			}
			bounds.Min.X, bounds.Min.Y = min(bounds.Min.X, x), min(bounds.Min.Y, y)
			bounds.Max.X, bounds.Max.Y = max(bounds.Max.X, x+1), max(bounds.Max.Y, y+1)
			if alpha == 0xffff {
				opaquePixels++
			}
		}
	}
	return bounds, !bounds.Empty() && opaquePixels == bounds.Dx()*bounds.Dy()
}

func roundIconCorners(dst *image.NRGBA, bounds image.Rectangle) {
	mask := image.NewAlpha(dst.Bounds())
	path := vector.NewRasterizer(mask.Bounds().Dx(), mask.Bounds().Dy())
	x0, y0, x1, y1 := float32(bounds.Min.X), float32(bounds.Min.Y), float32(bounds.Max.X), float32(bounds.Max.Y)
	radius := float32(min(bounds.Dx(), bounds.Dy())) * iconCornerRatio
	control := radius * iconCurveControl
	path.MoveTo(x0+radius, y0)
	path.LineTo(x1-radius, y0)
	path.CubeTo(x1-radius+control, y0, x1, y0+radius-control, x1, y0+radius)
	path.LineTo(x1, y1-radius)
	path.CubeTo(x1, y1-radius+control, x1-radius+control, y1, x1-radius, y1)
	path.LineTo(x0+radius, y1)
	path.CubeTo(x0+radius-control, y1, x0, y1-radius+control, x0, y1-radius)
	path.LineTo(x0, y0+radius)
	path.CubeTo(x0, y0+radius-control, x0+radius-control, y0, x0+radius, y0)
	path.ClosePath()
	path.Draw(mask, mask.Bounds(), image.White, image.Point{})
	draw.DrawMask(dst, bounds, dst, bounds.Min, mask, bounds.Min, draw.Src)
}
