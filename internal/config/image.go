// [INPUT]: The current MyGo locale, a user-selected PNG/JPEG, or a PNG saved in config.
// [OUTPUT]: Platform icon capability, bounded decoding, and an aspect-preserving transparent square image independent of the original path.
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
)

const (
	IconSupported    = runtime.GOOS == "darwin" || runtime.GOOS == "windows"
	iconSize         = 512
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
	dst := image.NewNRGBA(image.Rect(0, 0, iconSize, iconSize))
	w, h := iconSize, iconSize
	if cfg.Width > cfg.Height {
		h = max(1, iconSize*cfg.Height/cfg.Width)
	} else {
		w = max(1, iconSize*cfg.Width/cfg.Height)
	}
	target := image.Rect((iconSize-w)/2, (iconSize-h)/2, (iconSize+w)/2, (iconSize+h)/2)
	// Preserve the full image size even when dimensions are odd.
	target.Max = target.Min.Add(image.Pt(w, h))
	draw.CatmullRom.Scale(dst, target, src, src.Bounds(), draw.Src, nil)
	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
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
