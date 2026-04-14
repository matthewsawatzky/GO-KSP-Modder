package handlers

import (
	"net/http"

	"ksp-moder/services"
)

// ListSaves returns all save games with backup information.
func (h *Handler) ListSaves(w http.ResponseWriter, r *http.Request) {
	ksp, ok := h.getKSPPath(w)
	if !ok {
		return
	}
	saves := services.ListSaves(ksp)
	writeJSON(w, http.StatusOK, saves)
}

// BackupSave creates a timestamped zip backup of the named save.
func (h *Handler) BackupSave(w http.ResponseWriter, r *http.Request) {
	ksp, ok := h.getKSPPath(w)
	if !ok {
		return
	}
	name := r.PathValue("name")
	result, err := services.BackupSave(ksp, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if result == nil {
		writeError(w, http.StatusNotFound, `Save "`+name+`" not found`)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"filename": result.Filename,
		"path":     result.Path,
		"size_mb":  result.SizeMB,
	})
}
