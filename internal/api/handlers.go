package api

import (
	"errors"
	"io"
	"net/http"

	apperrors "backendassessment/internal/errors"
	"backendassessment/internal/storage"
)

func (h *Handler) handleEndpoint1(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperrors.Write(w, apperrors.MethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperrors.Write(w, apperrors.MethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request_counts": h.metering.Snapshot(),
		"total_requests": h.metering.Total(),
		"max_requests":   h.metering.Limit(),
	})
}

func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		apperrors.Write(w, apperrors.MethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		apperrors.Write(w, apperrors.InvalidUpload)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		apperrors.Write(w, apperrors.InvalidUpload)
		return
	}
	defer file.Close()

	size, err := uploadedSize(file)
	if err != nil {
		apperrors.Write(w, apperrors.Internal)
		return
	}

	if err := h.storage.TryAdd(size); err != nil {
		if errors.Is(err, storage.ErrLimitExceeded) {
			apperrors.Write(w, apperrors.StorageLimitExceeded)
			return
		}
		apperrors.Write(w, apperrors.Internal)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]uint64{
		"uploaded_bytes": size,
		"used_bytes":     h.storage.Used(),
		"max_bytes":      h.storage.Limit(),
	})
}

func (h *Handler) handleStorage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		apperrors.Write(w, apperrors.MethodNotAllowed)
		return
	}

	writeJSON(w, http.StatusOK, map[string]uint64{
		"used_bytes": h.storage.Used(),
		"max_bytes":  h.storage.Limit(),
	})
}

func uploadedSize(reader io.Reader) (uint64, error) {
	n, err := io.Copy(io.Discard, reader)
	if err != nil {
		return 0, err
	}
	return uint64(n), nil
}
