package media

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"

	"golang.org/x/image/draw"
)

const faviconEdgePx = 64

// EncodeFaviconICO converts supported raster image bytes into a compressed ICO payload.
// The resulting icon stores a single 64x64 PNG frame inside the ICO container.
func EncodeFaviconICO(raw []byte, inputMIME string) ([]byte, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty image")
	}
	img, err := decodeInput(raw, inputMIME)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	resized := resizeToSquare(img, faviconEdgePx)

	var pngBuf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&pngBuf, resized); err != nil {
		return nil, fmt.Errorf("encode png: %w", err)
	}
	pngData := pngBuf.Bytes()

	out := bytes.NewBuffer(make([]byte, 0, 6+16+len(pngData)))

	// ICONDIR: reserved(2)=0, type(2)=1(icon), count(2)=1.
	_ = binary.Write(out, binary.LittleEndian, uint16(0))
	_ = binary.Write(out, binary.LittleEndian, uint16(1))
	_ = binary.Write(out, binary.LittleEndian, uint16(1))

	// ICONDIRENTRY (single image).
	out.WriteByte(byte(faviconEdgePx)) // width (0 means 256)
	out.WriteByte(byte(faviconEdgePx)) // height
	out.WriteByte(0)                   // color count (0 for >=8bpp)
	out.WriteByte(0)                   // reserved
	_ = binary.Write(out, binary.LittleEndian, uint16(1))
	_ = binary.Write(out, binary.LittleEndian, uint16(32))
	_ = binary.Write(out, binary.LittleEndian, uint32(len(pngData)))
	_ = binary.Write(out, binary.LittleEndian, uint32(6+16))

	if _, err := out.Write(pngData); err != nil {
		return nil, fmt.Errorf("write ico payload: %w", err)
	}
	return out.Bytes(), nil
}

func resizeToSquare(src image.Image, edge int) image.Image {
	if src == nil || edge <= 0 {
		return src
	}
	dst := image.NewNRGBA(image.Rect(0, 0, edge, edge))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	return dst
}
