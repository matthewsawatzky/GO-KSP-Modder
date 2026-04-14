package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"ksp-moder/services"
)

// ListMods returns all mods with conflict and note data, plus total GameData size.
func (h *Handler) ListMods(w http.ResponseWriter, r *http.Request) {
	ksp, ok := h.getKSPPath(w)
	if !ok {
		return
	}

	mods := services.ListMods(ksp)
	conflicts := services.DetectConflicts(ksp)
	for i := range mods {
		if c, ok := conflicts[mods[i].Name]; ok {
			mods[i].Conflicts = c
		}
	}

	cfg, _ := h.cfg.Load()
	notes := cfg.ModNotes
	for i := range mods {
		mods[i].Note = notes[mods[i].Name]
	}

	// Total size of GameData.
	var totalSize int64
	gamedata := filepath.Join(ksp, "GameData")
	filepath.Walk(gamedata, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	totalMB := float64(int(float64(totalSize)/1024/1024*100)) / 100

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"mods":          mods,
		"total_count":   len(mods),
		"total_size_mb": totalMB,
	})
}

// AddMod handles a .zip file upload and extracts it into GameData.
func (h *Handler) AddMod(w http.ResponseWriter, r *http.Request) {
	ksp, ok := h.getKSPPath(w)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(200 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "Failed to parse upload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "No file uploaded")
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		writeError(w, http.StatusBadRequest, "Only .zip files are supported")
		return
	}

	added, err := services.AddMod(ksp, file, header.Filename)
	if err != nil {
		if os.IsPermission(err) {
			writeError(w, http.StatusForbidden, "Permission denied writing to GameData")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "added": added})
}

// RemoveMod deletes a mod directory.
func (h *Handler) RemoveMod(w http.ResponseWriter, r *http.Request) {
	ksp, ok := h.getKSPPath(w)
	if !ok {
		return
	}
	name := r.PathValue("name")
	result, err := services.RemoveMod(ksp, name)
	if err != nil {
		if os.IsPermission(err) {
			writeError(w, http.StatusForbidden, "Permission denied")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	if result == nil {
		writeError(w, http.StatusNotFound, `Mod "`+name+`" not found`)
		return
	}
	var warning *string
	if result.Large {
		s := "Removed large mod (" + floatStr(result.SizeMB) + " MB)"
		warning = &s
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"name":    result.Name,
		"size_mb": result.SizeMB,
		"large":   result.Large,
		"warning": warning,
	})
}

// ToggleMod enables or disables a mod.
func (h *Handler) ToggleMod(w http.ResponseWriter, r *http.Request) {
	ksp, ok := h.getKSPPath(w)
	if !ok {
		return
	}
	name := r.PathValue("name")
	result, err := services.ToggleMod(ksp, name)
	if err != nil {
		if os.IsPermission(err) {
			writeError(w, http.StatusForbidden, "Permission denied")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	if result == nil {
		writeError(w, http.StatusNotFound, `Mod "`+name+`" not found`)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "name": result.Name, "enabled": result.Enabled})
}

// BulkAction performs enable, disable, or remove on multiple mods.
func (h *Handler) BulkAction(w http.ResponseWriter, r *http.Request) {
	ksp, ok := h.getKSPPath(w)
	if !ok {
		return
	}
	var req struct {
		Action string   `json:"action"`
		Mods   []string `json:"mods"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Action == "" || req.Mods == nil {
		writeError(w, http.StatusBadRequest, "Missing action or mods list")
		return
	}

	gamedata := filepath.Join(ksp, "GameData")
	var affected, errors []string

	for _, name := range req.Mods {
		switch req.Action {
		case "enable":
			disabled := filepath.Join(gamedata, name+".disabled")
			enabled := filepath.Join(gamedata, name)
			if isDir(disabled) {
				if err := os.Rename(disabled, enabled); err != nil {
					errors = append(errors, name+": "+err.Error())
					continue
				}
			} else if !isDir(enabled) {
				errors = append(errors, name+": not found")
				continue
			}
			affected = append(affected, name)
		case "disable":
			enabled := filepath.Join(gamedata, name)
			disabled := filepath.Join(gamedata, name+".disabled")
			if isDir(enabled) && !strings.HasSuffix(name, ".disabled") {
				if err := os.Rename(enabled, disabled); err != nil {
					errors = append(errors, name+": "+err.Error())
					continue
				}
			} else if !isDir(disabled) {
				errors = append(errors, name+": not found")
				continue
			}
			affected = append(affected, name)
		case "remove":
			result, err := services.RemoveMod(ksp, name)
			if err != nil {
				errors = append(errors, name+": "+err.Error())
			} else if result == nil {
				errors = append(errors, name+": not found")
			} else {
				affected = append(affected, name)
			}
		default:
			writeError(w, http.StatusBadRequest, "Unknown action: "+req.Action)
			return
		}
	}

	if affected == nil {
		affected = []string{}
	}
	if errors == nil {
		errors = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"action":   req.Action,
		"affected": affected,
		"errors":   errors,
	})
}

// GetNotes returns all mod notes from config.
func (h *Handler) GetNotes(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.cfg.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg.ModNotes)
}

// SetNote saves or clears a note for a specific mod.
func (h *Handler) SetNote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mod  string `json:"mod"`
		Note string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Mod == "" {
		writeError(w, http.StatusBadRequest, "Missing mod name")
		return
	}
	cfg, err := h.cfg.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	note := strings.TrimSpace(req.Note)
	if note != "" {
		cfg.ModNotes[req.Mod] = note
	} else {
		delete(cfg.ModNotes, req.Mod)
	}
	if err := h.cfg.Save(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// ListProfiles returns all saved mod profiles.
func (h *Handler) ListProfiles(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.cfg.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg.Profiles)
}

// SaveProfile saves the current set of enabled mods as a named profile.
func (h *Handler) SaveProfile(w http.ResponseWriter, r *http.Request) {
	ksp, ok := h.getKSPPath(w)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "Missing profile name")
		return
	}
	name := strings.TrimSpace(req.Name)
	enabled := services.GetEnabledModNames(ksp)

	cfg, err := h.cfg.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	cfg.Profiles[name] = enabled
	if err := h.cfg.Save(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "name": name, "mods": enabled})
}

// LoadProfile applies a saved profile to the current install.
func (h *Handler) LoadProfile(w http.ResponseWriter, r *http.Request) {
	ksp, ok := h.getKSPPath(w)
	if !ok {
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "Missing profile name")
		return
	}
	cfg, err := h.cfg.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	mods, exists := cfg.Profiles[req.Name]
	if !exists {
		writeError(w, http.StatusNotFound, `Profile "`+req.Name+`" not found`)
		return
	}
	if err := services.ApplyProfile(ksp, mods); err != nil {
		if os.IsPermission(err) {
			writeError(w, http.StatusForbidden, "Permission denied")
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "name": req.Name})
}

// DeleteProfile removes a profile from config.
func (h *Handler) DeleteProfile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	cfg, err := h.cfg.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, exists := cfg.Profiles[name]; !exists {
		writeError(w, http.StatusNotFound, `Profile "`+name+`" not found`)
		return
	}
	delete(cfg.Profiles, name)
	if err := h.cfg.Save(cfg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// isDir reports whether path is an existing directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// floatStr converts a float64 to a trimmed decimal string.
func floatStr(f float64) string {
	return strings.TrimRight(strings.TrimRight(
		fmt.Sprintf("%.2f", f), "0"), ".")
}
