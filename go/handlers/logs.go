package handlers

import (
	"net/http"

	"ksp-modder/services"
)

var validFilters = map[string]bool{
	"all":            true,
	"errors":         true,
	"warnings":       true,
	"errors+warnings": true,
}

// GetLogs reads the game log file and returns filtered lines.
func (h *Handler) GetLogs(w http.ResponseWriter, r *http.Request) {
	ksp, ok := h.getKSPPath(w)
	if !ok {
		return
	}
	filter := r.URL.Query().Get("filter")
	if !validFilters[filter] {
		filter = "all"
	}
	result := services.ReadLog(ksp, filter)
	if result.Error != nil && len(result.Lines) == 0 {
		writeJSON(w, http.StatusNotFound, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// GetModErrors scans the log and returns errors grouped by mod name.
func (h *Handler) GetModErrors(w http.ResponseWriter, r *http.Request) {
	ksp, ok := h.getKSPPath(w)
	if !ok {
		return
	}
	mods := services.ListMods(ksp)
	names := make([]string, len(mods))
	for i, m := range mods {
		names[i] = m.Name
	}
	result := services.ScanModErrors(ksp, names)
	if result.Error != nil && len(result.Results) == 0 {
		writeJSON(w, http.StatusNotFound, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
