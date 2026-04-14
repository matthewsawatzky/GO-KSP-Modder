package services

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var modSkip = map[string]bool{
	"Squad":          true,
	"SquadExpansion": true,
	"Squad Expansion": true,
}

// Conflict describes a file shared between two or more mods.
type Conflict struct {
	File       string   `json:"file"`
	SharedWith []string `json:"shared_with"`
}

// Mod represents a single mod entry from GameData.
type Mod struct {
	Name      string     `json:"name"`
	Folder    string     `json:"folder"`
	Enabled   bool       `json:"enabled"`
	SizeMB    float64    `json:"size_mb"`
	Version   *string    `json:"version"`
	Conflicts []Conflict `json:"conflicts"`
	Note      string     `json:"note"`
}

func parseVersionFile(modPath string) *string {
	entries, err := os.ReadDir(modPath)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".version") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(modPath, e.Name()))
		if err != nil {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal(data, &obj); err != nil {
			continue
		}
		var ver interface{}
		if v, ok := obj["VERSION"]; ok {
			ver = v
		} else if v, ok := obj["version"]; ok {
			ver = v
		}
		if ver == nil {
			continue
		}
		switch v := ver.(type) {
		case string:
			s := v
			return &s
		case map[string]interface{}:
			major := intFromAny(v, "MAJOR", "major")
			minor := intFromAny(v, "MINOR", "minor")
			patch := intFromAny(v, "PATCH", "patch")
			s := fmt.Sprintf("%d.%d.%d", major, minor, patch)
			return &s
		}
	}
	return nil
}

func intFromAny(m map[string]interface{}, keys ...string) int {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if f, ok := v.(float64); ok {
				return int(f)
			}
		}
	}
	return 0
}

// ListMods returns all mods from GameData (excluding Squad directories).
func ListMods(kspPath string) []Mod {
	gamedata := filepath.Join(kspPath, "GameData")
	entries, err := os.ReadDir(gamedata)
	if err != nil {
		return []Mod{}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	var mods []Mod
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		baseName := name
		enabled := true
		if strings.HasSuffix(name, ".disabled") {
			baseName = name[:len(name)-9]
			enabled = false
		}
		if modSkip[baseName] {
			continue
		}
		entryPath := filepath.Join(gamedata, name)
		mods = append(mods, Mod{
			Name:      baseName,
			Folder:    name,
			Enabled:   enabled,
			SizeMB:    toMB(folderSize(entryPath)),
			Version:   parseVersionFile(entryPath),
			Conflicts: []Conflict{},
			Note:      "",
		})
	}
	if mods == nil {
		return []Mod{}
	}
	return mods
}

// DetectConflicts finds files shared between multiple enabled mods.
func DetectConflicts(kspPath string) map[string][]Conflict {
	gamedata := filepath.Join(kspPath, "GameData")
	entries, err := os.ReadDir(gamedata)
	if err != nil {
		return map[string][]Conflict{}
	}

	fileOwners := make(map[string][]string)
	for _, e := range entries {
		if !e.IsDir() || strings.HasSuffix(e.Name(), ".disabled") {
			continue
		}
		modName := e.Name()
		modPath := filepath.Join(gamedata, modName)
		filepath.Walk(modPath, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(modPath, p)
			fileOwners[rel] = append(fileOwners[rel], modName)
			return nil
		})
	}

	conflicts := make(map[string][]Conflict)
	for relPath, owners := range fileOwners {
		if len(owners) <= 1 {
			continue
		}
		for _, owner := range owners {
			var sharedWith []string
			for _, o := range owners {
				if o != owner {
					sharedWith = append(sharedWith, o)
				}
			}
			conflicts[owner] = append(conflicts[owner], Conflict{File: relPath, SharedWith: sharedWith})
		}
	}
	return conflicts
}

// ToggleResult is returned by ToggleMod.
type ToggleResult struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// ToggleMod enables or disables a mod by renaming its directory.
// Returns nil, nil when the mod is not found.
func ToggleMod(kspPath, modName string) (*ToggleResult, error) {
	gamedata := filepath.Join(kspPath, "GameData")
	enabledPath := filepath.Join(gamedata, modName)
	disabledPath := filepath.Join(gamedata, modName+".disabled")

	if info, err := os.Stat(enabledPath); err == nil && info.IsDir() {
		if err := os.Rename(enabledPath, disabledPath); err != nil {
			return nil, err
		}
		return &ToggleResult{Name: modName, Enabled: false}, nil
	}
	if info, err := os.Stat(disabledPath); err == nil && info.IsDir() {
		if err := os.Rename(disabledPath, enabledPath); err != nil {
			return nil, err
		}
		return &ToggleResult{Name: modName, Enabled: true}, nil
	}
	return nil, nil
}

// RemoveResult is returned by RemoveMod.
type RemoveResult struct {
	Name   string  `json:"name"`
	SizeMB float64 `json:"size_mb"`
	Large  bool    `json:"large"`
}

// RemoveMod deletes the mod directory (enabled or disabled).
// Returns nil, nil when the mod is not found.
func RemoveMod(kspPath, modName string) (*RemoveResult, error) {
	gamedata := filepath.Join(kspPath, "GameData")
	for _, suffix := range []string{"", ".disabled"} {
		path := filepath.Join(gamedata, modName+suffix)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			sizeMB := toMB(folderSize(path))
			if err := os.RemoveAll(path); err != nil {
				return nil, err
			}
			return &RemoveResult{Name: modName, SizeMB: sizeMB, Large: sizeMB > 50}, nil
		}
	}
	return nil, nil
}

// AddMod extracts a .zip upload into GameData, returning the names of added mod folders.
func AddMod(kspPath string, fileReader io.Reader, filename string) ([]string, error) {
	gamedata := filepath.Join(kspPath, "GameData")
	if err := os.MkdirAll(gamedata, 0755); err != nil {
		return nil, err
	}

	tmpDir, err := os.MkdirTemp("", "ksp-mod-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	// Save uploaded zip to a temp file so archive/zip can seek in it.
	zipPath := filepath.Join(tmpDir, "mod.zip")
	if err := writeReader(fileReader, zipPath); err != nil {
		return nil, err
	}

	extractDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return nil, err
	}
	if err := extractZip(zipPath, extractDir); err != nil {
		return nil, err
	}

	return installExtracted(extractDir, gamedata, filename)
}

func writeReader(r io.Reader, dst string) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func extractZip(zipPath, destDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()

	for _, f := range zr.File {
		// Sanitise path to prevent zip-slip.
		target := filepath.Join(destDir, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := extractZipFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func extractZipFile(f *zip.File, dst string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

// installExtracted mirrors the Python add_mod logic for finding GameData and copying mod folders.
func installExtracted(extractDir, gamedata, filename string) ([]string, error) {
	// Locate the effective GameData source directory.
	gdSrc := filepath.Join(extractDir, "GameData")
	if !isDir(gdSrc) {
		// Try one level deeper: extractDir/<single-dir>/GameData
		entries, _ := os.ReadDir(extractDir)
		if len(entries) == 1 && entries[0].IsDir() {
			inner := filepath.Join(extractDir, entries[0].Name(), "GameData")
			if isDir(inner) {
				gdSrc = inner
			}
		}
	}

	var added []string
	if isDir(gdSrc) {
		entries, err := os.ReadDir(gdSrc)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			src := filepath.Join(gdSrc, e.Name())
			dst := filepath.Join(gamedata, e.Name())
			if e.IsDir() {
				os.RemoveAll(dst)
				if err := copyDir(src, dst); err != nil {
					return nil, err
				}
			} else {
				if err := copyFile(src, dst); err != nil {
					return nil, err
				}
			}
			added = append(added, e.Name())
		}
	} else {
		// No GameData found: check for a single top-level directory.
		entries, _ := os.ReadDir(extractDir)
		if len(entries) == 1 && isDir(filepath.Join(extractDir, entries[0].Name())) {
			src := filepath.Join(extractDir, entries[0].Name())
			dst := filepath.Join(gamedata, entries[0].Name())
			os.RemoveAll(dst)
			if err := copyDir(src, dst); err != nil {
				return nil, err
			}
			added = append(added, entries[0].Name())
		} else {
			// Fallback: use the zip filename as the mod folder name.
			base := strings.TrimSuffix(filepath.Base(filename), ".zip")
			dst := filepath.Join(gamedata, base)
			os.RemoveAll(dst)
			if err := copyDir(extractDir, dst); err != nil {
				return nil, err
			}
			added = append(added, base)
		}
	}

	if added == nil {
		return []string{}, nil
	}
	return added, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(p, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

// GetEnabledModNames returns the names of currently-enabled mods (sorted).
func GetEnabledModNames(kspPath string) []string {
	gamedata := filepath.Join(kspPath, "GameData")
	entries, err := os.ReadDir(gamedata)
	if err != nil {
		return []string{}
	}
	var enabled []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasSuffix(e.Name(), ".disabled") || modSkip[e.Name()] {
			continue
		}
		enabled = append(enabled, e.Name())
	}
	sort.Strings(enabled)
	if enabled == nil {
		return []string{}
	}
	return enabled
}

// ApplyProfile enables mods in enabledList and disables everything else.
func ApplyProfile(kspPath string, enabledList []string) error {
	gamedata := filepath.Join(kspPath, "GameData")
	entries, err := os.ReadDir(gamedata)
	if err != nil {
		return err
	}
	want := make(map[string]bool, len(enabledList))
	for _, name := range enabledList {
		want[name] = true
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		path := filepath.Join(gamedata, name)
		if strings.HasSuffix(name, ".disabled") {
			base := name[:len(name)-9]
			if modSkip[base] {
				continue
			}
			if want[base] {
				os.Rename(path, filepath.Join(gamedata, base))
			}
		} else {
			if modSkip[name] {
				continue
			}
			if !want[name] {
				os.Rename(path, filepath.Join(gamedata, name+".disabled"))
			}
		}
	}
	return nil
}
