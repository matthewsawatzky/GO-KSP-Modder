package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"ksp-modder/services"
)

// GetInfo returns game version, disk usage, and install path.
func (h *Handler) GetInfo(w http.ResponseWriter, r *http.Request) {
	ksp, ok := h.getKSPPath(w)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version":    services.GetGameVersion(ksp),
		"disk_usage": services.GetDiskUsage(ksp),
		"path":       ksp,
	})
}

// ListScreenshots returns metadata for all screenshots.
func (h *Handler) ListScreenshots(w http.ResponseWriter, r *http.Request) {
	ksp, ok := h.getKSPPath(w)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, services.ListScreenshots(ksp))
}

// ServeScreenshot serves a single screenshot image file.
func (h *Handler) ServeScreenshot(w http.ResponseWriter, r *http.Request) {
	ksp, ok := h.getKSPPath(w)
	if !ok {
		return
	}
	// filepath.Base prevents path traversal (e.g. "../../etc/passwd" → "passwd").
	filename := filepath.Base(r.PathValue("filename"))
	ssDir := filepath.Join(ksp, "Screenshots")

	if info, err := os.Stat(ssDir); err != nil || !info.IsDir() {
		writeError(w, http.StatusNotFound, "Screenshots folder not found")
		return
	}

	filePath := filepath.Join(ssDir, filename)
	// Double-check the resolved path stays inside ssDir.
	if !strings.HasPrefix(filepath.Clean(filePath), filepath.Clean(ssDir)+string(os.PathSeparator)) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	http.ServeFile(w, r, filePath)
}

// ListCrafts returns craft files from Ships/VAB and Ships/SPH.
func (h *Handler) ListCrafts(w http.ResponseWriter, r *http.Request) {
	ksp, ok := h.getKSPPath(w)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, services.ListCrafts(ksp))
}
