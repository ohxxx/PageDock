// [INPUT]: A validated PNG icon.
// [OUTPUT]: A multiresolution macOS ICNS shared by the app and system notifications.
// [POS]: Icon encoding using MyGo's PNG ICNS types rather than legacy ARGB types.
// [PROTOCOL]: Update this header and internal/appearance/AGENTS.md.
package appearance

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"

	"golang.org/x/image/draw"
)

func EncodeICNS(data []byte) ([]byte, error) {
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var body bytes.Buffer
	for _, entry := range []struct {
		kind string
		size int
	}{
		{"ic11", 32}, {"ic12", 64}, {"ic07", 128}, {"ic13", 256}, {"ic08", 256}, {"ic14", 512}, {"ic09", 512}, {"ic10", 1024},
	} {
		dst := image.NewNRGBA(image.Rect(0, 0, entry.size, entry.size))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
		var payload bytes.Buffer
		if err := png.Encode(&payload, dst); err != nil {
			return nil, err
		}
		body.WriteString(entry.kind)
		_ = binary.Write(&body, binary.BigEndian, uint32(payload.Len()+8))
		body.Write(payload.Bytes())
	}
	var out bytes.Buffer
	out.WriteString("icns")
	_ = binary.Write(&out, binary.BigEndian, uint32(body.Len()+8))
	out.Write(body.Bytes())
	return out.Bytes(), nil
}
