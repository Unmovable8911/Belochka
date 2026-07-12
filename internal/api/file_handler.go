package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

const maxKeyFileSize = 32 << 10 // 32 KB

// HandleUploadKey handles POST /api/files/key.
func HandleUploadKey(keyDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxKeyFileSize)

		if err := r.ParseMultipartForm(maxKeyFileSize); err != nil {
			writeKeyUploadError(w, http.StatusRequestEntityTooLarge, "file_too_large", "File exceeds maximum size of 32 KB")
			return
		}

		file, _, err := r.FormFile("keyfile")
		if err != nil {
			writeKeyUploadError(w, http.StatusBadRequest, "missing_field", "No file provided in 'keyfile' field")
			return
		}
		defer file.Close()

		keyBytes := make([]byte, maxKeyFileSize)
		n, err := file.Read(keyBytes)
		if err != nil && err.Error() != "EOF" {
			writeKeyUploadError(w, http.StatusBadRequest, "read_error", "Failed to read uploaded file")
			return
		}
		keyBytes = keyBytes[:n]

		// Validate the private key before writing to disk.
		if _, err := ssh.ParsePrivateKey(keyBytes); err != nil {
			writeKeyUploadError(w, http.StatusUnprocessableEntity, "invalid_key", "Uploaded file is not a valid SSH private key")
			return
		}

		if err := os.MkdirAll(keyDir, 0700); err != nil {
			slog.Error("failed to create key directory", "dir", keyDir, "error", err)
			writeKeyUploadError(w, http.StatusInternalServerError, "internal", "Failed to create key storage")
			return
		}

		filename := uuid.New().String() + ".key"
		filepath := filepath.Join(keyDir, filename)

		if err := os.WriteFile(filepath, keyBytes, 0600); err != nil {
			slog.Error("failed to write key file", "path", filepath, "error", err)
			writeKeyUploadError(w, http.StatusInternalServerError, "internal", "Failed to store key file")
			return
		}

		slog.Info("key file uploaded", "path", filepath)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"path": filepath,
		})
	}
}

func writeKeyUploadError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

