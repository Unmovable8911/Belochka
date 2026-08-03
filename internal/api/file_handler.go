package api

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"belochka/internal/httpx"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

const maxKeyFileSize = 32 << 10 // 32 KB

// HandleUploadKey handles POST /api/files/key.
func HandleUploadKey(keyDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxKeyFileSize)

		if err := r.ParseMultipartForm(maxKeyFileSize); err != nil {
			httpx.WriteError(w,http.StatusRequestEntityTooLarge, "file_too_large", "File exceeds maximum size of 32 KB")
			return
		}

		file, _, err := r.FormFile("keyfile")
		if err != nil {
			httpx.WriteError(w,http.StatusBadRequest, "missing_field", "No file provided in 'keyfile' field")
			return
		}
		defer file.Close()

		keyBytes := make([]byte, maxKeyFileSize)
		n, err := file.Read(keyBytes)
		if err != nil && err.Error() != "EOF" {
			httpx.WriteError(w,http.StatusBadRequest, "read_error", "Failed to read uploaded file")
			return
		}
		keyBytes = keyBytes[:n]

		// Validate the private key before writing to disk.
		if _, err := ssh.ParsePrivateKey(keyBytes); err != nil {
			httpx.WriteError(w,http.StatusUnprocessableEntity, "invalid_key", "Uploaded file is not a valid SSH private key")
			return
		}

		if err := os.MkdirAll(keyDir, 0700); err != nil {
			slog.Error("failed to create key directory", "dir", keyDir, "error", err)
			httpx.WriteError(w,http.StatusInternalServerError, "internal", "Failed to create key storage")
			return
		}

		filename := uuid.New().String() + ".key"
		filepath := filepath.Join(keyDir, filename)

		if err := os.WriteFile(filepath, keyBytes, 0600); err != nil {
			slog.Error("failed to write key file", "path", filepath, "error", err)
			httpx.WriteError(w,http.StatusInternalServerError, "internal", "Failed to store key file")
			return
		}

		slog.Info("key file uploaded", "path", filepath)

		httpx.WriteJSON(w, http.StatusOK, map[string]string{
			"path": filepath,
		})
	}
}

