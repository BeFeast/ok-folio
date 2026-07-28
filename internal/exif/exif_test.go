package exif

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"math"
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

func TestDecodeEmbeddedMetadataReadsValidGPS(t *testing.T) {
	// 52 deg 30' 0" N, 13 deg 24' 0" E.
	imageBytes := jpegWithExif(t, gpsExifTIFF(t,
		[3][2]uint32{{52, 1}, {30, 1}, {0, 1}},
		[3][2]uint32{{13, 1}, {24, 1}, {0, 1}},
	))

	metadata, err := DecodeEmbeddedMetadata(bytes.NewReader(imageBytes))
	if err != nil {
		t.Fatalf("DecodeEmbeddedMetadata failed: %v", err)
	}
	if metadata.GPSLatitude == nil || metadata.GPSLongitude == nil {
		t.Fatalf("expected GPS coordinates to be kept, got %#v", metadata)
	}
	if *metadata.GPSLatitude != 52.5 || *metadata.GPSLongitude != 13.4 {
		t.Fatalf("expected 52.5/13.4, got %v/%v", *metadata.GPSLatitude, *metadata.GPSLongitude)
	}
}

func TestDecodeEmbeddedMetadataDropsNaNGPS(t *testing.T) {
	// Zero denominators make goexif divide 0 by 0, yielding NaN without an
	// error. Such a value must never reach the database: encoding/json cannot
	// marshal it, so one bad row empties every response that contains it.
	imageBytes := jpegWithExif(t, gpsExifTIFF(t,
		[3][2]uint32{{0, 0}, {0, 0}, {0, 0}},
		[3][2]uint32{{0, 0}, {0, 0}, {0, 0}},
	))

	metadata, err := DecodeEmbeddedMetadata(bytes.NewReader(imageBytes))
	if err != nil {
		t.Fatalf("DecodeEmbeddedMetadata failed: %v", err)
	}
	if metadata.GPSLatitude != nil || metadata.GPSLongitude != nil {
		t.Fatalf("expected NaN GPS to be dropped, got lat=%v lon=%v",
			*metadata.GPSLatitude, *metadata.GPSLongitude)
	}

	if _, err := json.Marshal(metadata); err != nil {
		t.Fatalf("metadata must stay JSON-encodable: %v", err)
	}
}

func TestDecodeEmbeddedMetadataDropsOutOfRangeGPS(t *testing.T) {
	// 200 degrees latitude is finite but not a real coordinate.
	imageBytes := jpegWithExif(t, gpsExifTIFF(t,
		[3][2]uint32{{200, 1}, {0, 1}, {0, 1}},
		[3][2]uint32{{13, 1}, {24, 1}, {0, 1}},
	))

	metadata, err := DecodeEmbeddedMetadata(bytes.NewReader(imageBytes))
	if err != nil {
		t.Fatalf("DecodeEmbeddedMetadata failed: %v", err)
	}
	if metadata.GPSLatitude != nil || metadata.GPSLongitude != nil {
		t.Fatalf("expected out-of-range GPS to be dropped, got %#v", metadata)
	}
}

func TestCoordinateValidation(t *testing.T) {
	nan := math.NaN()
	inf := math.Inf(1)

	for _, tc := range []struct {
		name string
		lat  float64
		lon  float64
		want bool
	}{
		{"zero", 0, 0, true},
		{"bounds", -90, 180, true},
		{"nan latitude", nan, 13.4, false},
		{"nan longitude", 52.5, nan, false},
		{"inf latitude", inf, 13.4, false},
		{"inf longitude", 52.5, -inf, false},
		{"latitude above range", 90.1, 13.4, false},
		{"longitude below range", 52.5, -180.1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validLatitude(tc.lat) && validLongitude(tc.lon); got != tc.want {
				t.Fatalf("lat=%v lon=%v: got %v, want %v", tc.lat, tc.lon, got, tc.want)
			}
		})
	}
}

// gpsExifTIFF builds a minimal TIFF whose IFD0 carries only a GPS sub-IFD with
// latitude/longitude as three RATIONAL pairs each (degrees, minutes, seconds).
func gpsExifTIFF(t *testing.T, lat, lon [3][2]uint32) []byte {
	t.Helper()
	const (
		ifd0Offset  = 8
		ifd0Size    = 2 + 1*12 + 4
		gpsOffset   = ifd0Offset + ifd0Size
		gpsEntries  = 4
		gpsSize     = 2 + gpsEntries*12 + 4
		latOffset   = gpsOffset + gpsSize
		ratTriplet  = 3 * 8
		lonOffset   = latOffset + ratTriplet
		totalLength = lonOffset + ratTriplet
	)

	buf := make([]byte, totalLength)
	copy(buf[0:2], "II")
	binary.LittleEndian.PutUint16(buf[2:4], 42)
	binary.LittleEndian.PutUint32(buf[4:8], ifd0Offset)

	binary.LittleEndian.PutUint16(buf[ifd0Offset:ifd0Offset+2], 1)
	writeIFDEntry(buf, ifd0Offset+2, 0x8825, 4, 1, gpsOffset)

	binary.LittleEndian.PutUint16(buf[gpsOffset:gpsOffset+2], gpsEntries)
	writeASCIIInline(buf, gpsOffset+2, 0x0001, "N")
	writeIFDEntry(buf, gpsOffset+14, 0x0002, 5, 3, latOffset)
	writeASCIIInline(buf, gpsOffset+26, 0x0003, "E")
	writeIFDEntry(buf, gpsOffset+38, 0x0004, 5, 3, lonOffset)

	writeRationals(buf, latOffset, lat)
	writeRationals(buf, lonOffset, lon)
	return buf
}

func writeASCIIInline(buf []byte, offset int, tag uint16, value string) {
	binary.LittleEndian.PutUint16(buf[offset:offset+2], tag)
	binary.LittleEndian.PutUint16(buf[offset+2:offset+4], 2)
	binary.LittleEndian.PutUint32(buf[offset+4:offset+8], uint32(len(value)+1))
	copy(buf[offset+8:offset+12], value)
}

func writeRationals(buf []byte, offset int, values [3][2]uint32) {
	for i, v := range values {
		at := offset + i*8
		binary.LittleEndian.PutUint32(buf[at:at+4], v[0])
		binary.LittleEndian.PutUint32(buf[at+4:at+8], v[1])
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
