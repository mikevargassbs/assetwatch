package acceptance

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

const uploadDir = "data/uploads/acceptance"

var unsafeFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

// allowedDocumentExtensions restricts uploads to document/image types so a
// malicious .html/.svg can't be stored and later served back to another
// user's browser.
var allowedDocumentExtensions = map[string]bool{
	".pdf": true, ".jpg": true, ".jpeg": true, ".png": true,
}

var ErrUnsupportedFileType = fmt.Errorf("unsupported file type")

// SaveUploadedDocument writes the scanned acceptance document to local disk
// and returns its stored path. Filenames are sanitized and prefixed with the
// unit ID so uploads never collide or traverse outside uploadDir.
func SaveUploadedDocument(unitID uuid.UUID, originalFilename string, src io.Reader) (string, error) {
	if !allowedDocumentExtensions[strings.ToLower(filepath.Ext(originalFilename))] {
		return "", ErrUnsupportedFileType
	}

	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return "", fmt.Errorf("create upload dir: %w", err)
	}

	safeName := unsafeFilenameChars.ReplaceAllString(filepath.Base(originalFilename), "_")
	if safeName == "" {
		safeName = "document"
	}
	storedPath := filepath.Join(uploadDir, fmt.Sprintf("%s-%s", unitID, safeName))

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
