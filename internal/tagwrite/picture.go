package tagwrite

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	_ "image/jpeg" // the covers' kinds, which pictures tell the size of
	_ "image/png"
)

// checkCover returns the MIME type of a cover, by its first bytes: a JPEG
// or PNG image of at most MaxCover bytes, whose size can be read.
func checkCover(p *Picture) (string, error) {
	if len(p.Data) == 0 || len(p.Data) > MaxCover {
		return "", ErrCover
	}
	mime := ""
	switch {
	case bytes.HasPrefix(p.Data, []byte("\xFF\xD8\xFF")):
		mime = "image/jpeg"
	case bytes.HasPrefix(p.Data, []byte("\x89PNG\r\n\x1a\n")):
		mime = "image/png"
	default:
		return "", ErrCover
	}
	if _, _, err := image.DecodeConfig(bytes.NewReader(p.Data)); err != nil {
		return "", ErrCover
	}
	return mime, nil
}

// pictureBlock is p as the front cover in a FLAC PICTURE block, which Ogg
// files hold in a comment too: its type, MIME type, an empty description,
// its width, height, bits per pixel and colours, and its data.
func pictureBlock(p *Picture) []byte {
	var width, height, depth, colors int
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(p.Data)); err == nil {
		width, height, depth = cfg.Width, cfg.Height, 24
		switch m := cfg.ColorModel.(type) {
		case color.Palette:
			depth, colors = 8, len(m)
		default:
			switch m {
			case color.GrayModel:
				depth = 8
			case color.Gray16Model:
				depth = 16
			case color.RGBAModel, color.NRGBAModel:
				depth = 32
			case color.RGBA64Model, color.NRGBA64Model:
				depth = 64
			}
		}
	}
	var b bytes.Buffer
	u32 := func(n int) { b.Write(binary.BigEndian.AppendUint32(nil, uint32(n))) }
	u32(3) // the front cover
	u32(len(p.MIME))
	b.WriteString(p.MIME)
	u32(0) // the description
	u32(width)
	u32(height)
	u32(depth)
	u32(colors)
	u32(len(p.Data))
	b.Write(p.Data)
	return b.Bytes()
}
