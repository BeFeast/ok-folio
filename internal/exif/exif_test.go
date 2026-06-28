package exif

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestDecodeEmbeddedMetadataReadsDimensionsAndExif(t *testing.T) {
	imageBytes := jpegWithExif(t, minimalExifTIFF(t))

	metadata, err := DecodeEmbeddedMetadata(bytes.NewReader(imageBytes))
	if err != nil {
		t.Fatalf("DecodeEmbeddedMetadata failed: %v", err)
	}
	if metadata.Width != 12 || metadata.Height != 8 {
		t.Fatalf("expected dimensions 12x8, got %dx%d", metadata.Width, metadata.Height)
	}
	if metadata.CapturedAt == nil || metadata.CapturedAt.Format("2006-01-02 15:04:05") != "2024-05-06 07:08:09" {
		t.Fatalf("expected DateTimeOriginal, got %v", metadata.CapturedAt)
	}
	if metadata.CameraMake != "Nikon" || metadata.CameraModel != "D90 Camera" {
		t.Fatalf("expected Nikon D90, got make=%q model=%q", metadata.CameraMake, metadata.CameraModel)
	}
	if metadata.LensModel != "35mm f/1.8" {
		t.Fatalf("expected lens model, got %q", metadata.LensModel)
	}
	if metadata.Orientation != "6" {
		t.Fatalf("expected orientation 6, got %q", metadata.Orientation)
	}
}

func TestDecodeEmbeddedMetadataIgnoresMissingExif(t *testing.T) {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 5, 3))
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}

	metadata, err := DecodeEmbeddedMetadata(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("DecodeEmbeddedMetadata failed: %v", err)
	}
	if metadata.Width != 5 || metadata.Height != 3 {
		t.Fatalf("expected dimensions 5x3, got %dx%d", metadata.Width, metadata.Height)
	}
	if metadata.CapturedAt != nil || metadata.CameraMake != "" || metadata.CameraModel != "" {
		t.Fatalf("expected missing EXIF to be empty, got %#v", metadata)
	}
}

func jpegWithExif(t *testing.T, tiff []byte) []byte {
	t.Helper()
	var jpegBuf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 12, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 12; x++ {
			img.Set(x, y, color.RGBA{R: uint8(20 * x), G: uint8(20 * y), B: 120, A: 255})
		}
	}
	if err := jpeg.Encode(&jpegBuf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	src := jpegBuf.Bytes()
	app1Data := append([]byte("Exif\x00\x00"), tiff...)
	if len(app1Data)+2 > 0xffff {
		t.Fatalf("APP1 payload too large")
	}
	var out bytes.Buffer
	out.Write(src[:2])
	out.Write([]byte{0xff, 0xe1, byte((len(app1Data) + 2) >> 8), byte(len(app1Data) + 2)})
	out.Write(app1Data)
	out.Write(src[2:])
	return out.Bytes()
}

func minimalExifTIFF(t *testing.T) []byte {
	t.Helper()
	const (
		ifd0Offset   = 8
		ifd0Entries  = 4
		ifd0Size     = 2 + ifd0Entries*12 + 4
		subIFDOffset = ifd0Offset + ifd0Size
		subEntries   = 2
		subIFDSize   = 2 + subEntries*12 + 4
		dataOffset   = subIFDOffset + subIFDSize
	)
	data := map[string][]byte{
		"make":     []byte("Nikon\x00"),
		"model":    []byte("D90 Camera\x00"),
		"captured": []byte("2024:05:06 07:08:09\x00"),
		"lens":     []byte("35mm f/1.8\x00"),
	}
	offsets := map[string]uint32{}
	next := uint32(dataOffset)
	for _, key := range []string{"make", "model", "captured", "lens"} {
		offsets[key] = next
		next += uint32(len(data[key]))
	}

	buf := make([]byte, next)
	copy(buf[0:2], "II")
	binary.LittleEndian.PutUint16(buf[2:4], 42)
	binary.LittleEndian.PutUint32(buf[4:8], ifd0Offset)

	binary.LittleEndian.PutUint16(buf[ifd0Offset:ifd0Offset+2], ifd0Entries)
	writeIFDEntry(buf, ifd0Offset+2, 0x010f, 2, uint32(len(data["make"])), offsets["make"])
	writeIFDEntry(buf, ifd0Offset+14, 0x0110, 2, uint32(len(data["model"])), offsets["model"])
	writeIFDEntry(buf, ifd0Offset+26, 0x0112, 3, 1, 6)
	writeIFDEntry(buf, ifd0Offset+38, 0x8769, 4, 1, subIFDOffset)

	binary.LittleEndian.PutUint16(buf[subIFDOffset:subIFDOffset+2], subEntries)
	writeIFDEntry(buf, subIFDOffset+2, 0x9003, 2, uint32(len(data["captured"])), offsets["captured"])
	writeIFDEntry(buf, subIFDOffset+14, 0xa434, 2, uint32(len(data["lens"])), offsets["lens"])

	for _, key := range []string{"make", "model", "captured", "lens"} {
		copy(buf[offsets[key]:], data[key])
	}
	return buf
}

func writeIFDEntry(buf []byte, offset int, tag uint16, typ uint16, count uint32, value uint32) {
	binary.LittleEndian.PutUint16(buf[offset:offset+2], tag)
	binary.LittleEndian.PutUint16(buf[offset+2:offset+4], typ)
	binary.LittleEndian.PutUint32(buf[offset+4:offset+8], count)
	if typ == 3 && count == 1 {
		binary.LittleEndian.PutUint16(buf[offset+8:offset+10], uint16(value))
		return
	}
	binary.LittleEndian.PutUint32(buf[offset+8:offset+12], value)
}
