package handlers

import (
	"encoding/json"
	"net/http"
	"os"

	"ksp-moder/services"
)

// DetectInstalls scans standard KSP locations and saves results to config.
func (h *Handler) DetectInstalls(w http.ResponseWriter, r *http.Request) {
	installs := services.DetectInstalls()

	cfg, err := h.cfg.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	paths := make([]string, len(installs))
	for i, inst := range installs {
		paths[i] = inst.Path
	}
	cfg.AllInstalls = paths
	h.cfg.Save(cfg) //nolint:errcheck

	writeJSON(w, http.StatusOK, installs)
}

// SetPath sets the active KSP installation path.
func (h *Handler) SetPath(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		writeError(w, http.StatusBadRequest, "Missing path")
		return
	}
	if !services.ValidateKSPInstall(req.Path) {
		writeError(w, http.StatusBadRequest, "Not a valid KSP install directory")
		return
	}
	cfg, err := h.cfg.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	cfg.KSPPath = &req.Path
	found := false
	for _, p := range cfg.AllInstalls {
		if p == req.Path {
			found = true
			break
		}
	}
	if !found {
		cfg.AllInstalls = append(cfg.AllInstalls, req.Path)
	}
	if err := h.cfg.Save(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "path": req.Path})
}

// CurrentPath returns the configured KSP path and whether it exists on disk.
func (h *Handler) CurrentPath(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.cfg.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var path *string
	exists := false
	if cfg.KSPPath != nil {
		path = cfg.KSPPath
		info, err := os.Stat(*cfg.KSPPath)
		exists = err == nil && info.IsDir()
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"path": path, "exists": exists})
}

// GetSettings returns the current settings, filling in defaults for missing keys.
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.cfg.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg.Settings)
}

// SaveSettings updates the allowed subset of settings.
func (h *Handler) SaveSettings(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req == nil {
		writeError(w, http.StatusBadRequest, "No settings provided")
		return
	}
	cfg, err := h.cfg.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if v, ok := req["accent_color"].(string); ok {
		cfg.Settings.AccentColor = v
	}
	if v, ok := req["log_lines"].(float64); ok {
		cfg.Settings.LogLines = int(v)
	}
	if v, ok := req["confirm_remove"].(bool); ok {
		cfg.Settings.ConfirmRemove = v
	}
	if v, ok := req["sort_mods_by"].(string); ok {
		cfg.Settings.SortModsBy = v
	}

	if err := h.cfg.Save(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "settings": cfg.Settings})
}
