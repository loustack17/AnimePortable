//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"bytes"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"

	fltk "github.com/pwiecz/go-fltk"
)

const (
	detailCoverMaxBytes  = 4 << 20
	detailCoverMaxPixels = 16 << 20
	detailCoverWidth     = 240
	detailCoverHeight    = 320
)

func decodeDetailCover(data []byte, mediaType string) ([]byte, error) {
	if len(data) == 0 || len(data) > detailCoverMaxBytes {
		return nil, errors.New("invalid cover size")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || config.Width > 8192 || config.Height > 8192 || config.Width*config.Height > detailCoverMaxPixels {
		return nil, errors.New("invalid cover dimensions")
	}
	if mediaType == "image/png" && format != "png" || mediaType == "image/jpeg" && format != "jpeg" || mediaType != "image/png" && mediaType != "image/jpeg" {
		return nil, errors.New("unsupported cover format")
	}
	decoded, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || format != "png" && format != "jpeg" {
		return nil, errors.New("invalid cover image")
	}
	pixels := make([]byte, detailCoverWidth*detailCoverHeight*4)
	source := decoded.Bounds()
	contentWidth, contentHeight := detailCoverWidth, detailCoverHeight
	if source.Dx()*contentHeight > source.Dy()*contentWidth {
		contentHeight = max(1, source.Dy()*contentWidth/source.Dx())
	} else {
		contentWidth = max(1, source.Dx()*contentHeight/source.Dy())
	}
	left, top := (detailCoverWidth-contentWidth)/2, (detailCoverHeight-contentHeight)/2
	for y := 0; y < contentHeight; y++ {
		sourceY := source.Min.Y + y*source.Dy()/contentHeight
		for x := 0; x < contentWidth; x++ {
			sourceX := source.Min.X + x*source.Dx()/contentWidth
			r, g, b, a := decoded.At(sourceX, sourceY).RGBA()
			offset := ((top+y)*detailCoverWidth + left + x) * 4
			pixels[offset] = uint8(r >> 8)
			pixels[offset+1] = uint8(g >> 8)
			pixels[offset+2] = uint8(b >> 8)
			pixels[offset+3] = uint8(a >> 8)
		}
	}
	return pixels, nil
}

func newDetailCoverImage(pixels []byte) (*fltk.RgbImage, error) {
	if len(pixels) != detailCoverWidth*detailCoverHeight*4 {
		return nil, errors.New("invalid decoded cover size")
	}
	rgb, err := fltk.NewRgbImage(pixels, detailCoverWidth, detailCoverHeight, 4)
	if err != nil {
		return nil, err
	}
	return rgb, nil
}
