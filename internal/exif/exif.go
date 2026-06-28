package exif

import (
	"bytes"
	"fmt"
	"image"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"ok-folio/internal/config"

	goexif "github.com/rwcarlsen/goexif/exif"

	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
)

type Metadata struct {
	Title      string
	Artist     string
	UploadDate time.Time
}

type EmbeddedMetadata struct {
	Width        int
	Height       int
	CapturedAt   *time.Time
	CameraMake   string
	CameraModel  string
	LensModel    string
	Orientation  string
	GPSLatitude  *float64
	GPSLongitude *float64
}

func ReadEmbeddedMetadata(filePath string) (EmbeddedMetadata, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return EmbeddedMetadata{}, err
	}
	defer file.Close()
	return DecodeEmbeddedMetadata(file)
}

func DecodeEmbeddedMetadata(r io.ReadSeeker) (EmbeddedMetadata, error) {
	var metadata EmbeddedMetadata
	cfg, _, err := image.DecodeConfig(r)
	if err != nil {
		return EmbeddedMetadata{}, err
	}
	metadata.Width = cfg.Width
	metadata.Height = cfg.Height

	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return metadata, nil
	}
	exifData, err := goexif.Decode(r)
	if err != nil {
		return metadata, nil
	}
	if capturedAt, err := exifData.DateTime(); err == nil && !capturedAt.IsZero() {
		capturedUTC := capturedAt.UTC()
		metadata.CapturedAt = &capturedUTC
	}
	metadata.CameraMake = exifString(exifData, goexif.Make)
	metadata.CameraModel = exifString(exifData, goexif.Model)
	metadata.LensModel = exifString(exifData, goexif.LensModel)
	metadata.Orientation = exifString(exifData, goexif.Orientation)
	if lat, lon, err := exifData.LatLong(); err == nil {
		metadata.GPSLatitude = &lat
		metadata.GPSLongitude = &lon
	}
	return metadata, nil
}

func exifString(exifData *goexif.Exif, field goexif.FieldName) string {
	tag, err := exifData.Get(field)
	if err != nil {
		return ""
	}
	value, err := tag.StringVal()
	if err == nil {
		return strings.TrimSpace(value)
	}
	return strings.Trim(strings.TrimSpace(tag.String()), `"`)
}

// SetMetadata sets EXIF metadata on an image file using exiftool
func SetMetadata(filePath string, metadata Metadata, cfg *config.EXIFConfig) error {
	if !isExiftoolAvailable() {
		return fmt.Errorf("exiftool not found in PATH")
	}

	args := []string{"-overwrite_original"}

	// Set Artist
	if cfg.SetArtist && metadata.Artist != "" {
		args = append(args, fmt.Sprintf("-Artist=%s", metadata.Artist))
	}

	// Set CreateDate
	if cfg.SetDate && !metadata.UploadDate.IsZero() {
		dateStr := metadata.UploadDate.Format("2006:01:02 15:04:05")
		args = append(args,
			fmt.Sprintf("-CreateDate=%s", dateStr),
			fmt.Sprintf("-DateTimeOriginal=%s", dateStr),
		)
	}

	// Set Title
	if cfg.SetTitle && metadata.Title != "" {
		modifiedTitle := fmt.Sprintf("%s by %s", metadata.Title, metadata.Artist)
		args = append(args,
			fmt.Sprintf("-Title=%s", modifiedTitle),
			fmt.Sprintf("-Description=%s", modifiedTitle),
		)
	}

	args = append(args, filePath)

	cmd := exec.Command("exiftool", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exiftool failed: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}

// GetMetadata reads EXIF metadata from an image file
func GetMetadata(filePath string) (map[string]string, error) {
	if !isExiftoolAvailable() {
		return nil, fmt.Errorf("exiftool not found in PATH")
	}

	cmd := exec.Command("exiftool", "-s", "-Artist", "-CreateDate", "-Title", filePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("exiftool read failed: %w", err)
	}

	metadata := make(map[string]string)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			metadata[key] = value
		}
	}

	return metadata, nil
}

// HasMetadata checks if an image file has EXIF metadata
func HasMetadata(filePath string, field string) (bool, error) {
	metadata, err := GetMetadata(filePath)
	if err != nil {
		return false, err
	}

	_, exists := metadata[field]
	return exists, nil
}

func isExiftoolAvailable() bool {
	// Check if exiftool exists in PATH
	_, err := exec.LookPath("exiftool")
	return err == nil
}

// InstallExiftool provides instructions for installing exiftool
func InstallExiftool() string {
	if _, err := os.Stat("/etc/alpine-release"); err == nil {
		return "apk add --no-cache exiftool"
	}
	if _, err := os.Stat("/etc/debian_version"); err == nil {
		return "apt-get update && apt-get install -y libimage-exiftool-perl"
	}
	return "Install exiftool: https://exiftool.org/"
}
