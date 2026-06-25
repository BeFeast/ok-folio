package exif

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"ok-folio/internal/config"
)

type Metadata struct {
	Title      string
	Artist     string
	UploadDate time.Time
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
