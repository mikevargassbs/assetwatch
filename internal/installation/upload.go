package installation

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

const photoUploadDir = "data/uploads/installation"

var unsafePhotoFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// allowedPhotoExtensions restricts uploads to image types so a malicious
// .html/.svg can't be stored and later served back to another user's browser.
var allowedPhotoExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".heic": true,
}

var ErrUnsupportedFileType = fmt.Errorf("unsupported file type")

// SaveUploadedPhoto writes an installation photo to local disk and returns
// its stored path. Filenames are sanitized and prefixed with the unit ID and
// a fresh UUID so uploads never collide or traverse outside photoUploadDir.
func SaveUploadedPhoto(unitID uuid.UUID, originalFilename string, src io.Reader) (string, error) {
	if !allowedPhotoExtensions[strings.ToLower(filepath.Ext(originalFilename))] {
		return "", ErrUnsupportedFileType
	}

	if err := os.MkdirAll(photoUploadDir, 0o755); err != nil {
		return "", fmt.Errorf("create upload dir: %w", err)
	}

	safeName := unsafePhotoFilenameChars.ReplaceAllString(filepath.Base(originalFilename), "_")
	if safeName == "" {
		safeName = "photo"
	}
	storedPath := filepath.Join(photoUploadDir, fmt.Sprintf("%s-%s-%s", unitID, uuid.NewString(), safeName))

	out, err := os.Create(storedPath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, src); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return storedPath, nil
}
