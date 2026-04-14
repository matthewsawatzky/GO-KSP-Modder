package handlers

import (
	"encoding/json"
	"net/http"
	"os"

	"ksp-moder/config"
)

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	cfg *config.Manager
}

// New creates a Handler backed by the given config manager.
func New(cfg *config.Manager) *Handler {
	return &Handler{cfg: cfg}
}

// writeJSON serialises v as JSON with the given HTTP status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// writeError writes a {"error": msg} JSON response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// getKSPPath loads the configured KSP path and verifies it exists on disk.
// It writes an error response and returns ("", false) on failure.
func (h *Handler) getKSPPath(w http.ResponseWriter) (string, bool) {
	cfg, err := h.cfg.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load config")
		return "", false
	}
	if cfg.KSPPath == nil {
		writeError(w, http.StatusBadRequest, "No KSP path configured")
		return "", false
	}
	ksp := *cfg.KSPPath
	info, err := os.Stat(ksp)
	if err != nil || !info.IsDir() {
		writeError(w, http.StatusBadRequest, "No KSP path configured")
		return "", false
	}
	return ksp, true
}
