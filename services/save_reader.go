package services

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Backup represents a save backup archive.
type Backup struct {
	Filename string  `json:"filename"`
	SizeMB   float64 `json:"size_mb"`
	Created  string  `json:"created"`
}

// Save represents a KSP save game.
type Save struct {
	Name     string   `json:"name"`
	SizeMB   float64  `json:"size_mb"`
	Modified string   `json:"modified"`
	Backups  []Backup `json:"backups"`
}

var saveSkip = map[string]bool{"backups": true, "scenarios": true, "training": true}

// ListSaves returns all user save games with their backup lists.
func ListSaves(kspPath string) []Save {
	savesDir := filepath.Join(kspPath, "saves")
	entries, err := os.ReadDir(savesDir)
	if err != nil {
		return []Save{}
	}

	var saves []Save
	for _, e := range entries {
		if !e.IsDir() || saveSkip[strings.ToLower(e.Name())] {
			continue
		}
		entryPath := filepath.Join(savesDir, e.Name())
		var modified string
		if info, err := e.Info(); err == nil {
			modified = info.ModTime().Format("2006-01-02 15:04:05")
		} else {
			modified = "Unknown"
		}
		saves = append(saves, Save{
			Name:     e.Name(),
			SizeMB:   toMB(folderSize(entryPath)),
			Modified: modified,
			Backups:  listBackupsForSave(kspPath, e.Name()),
		})
	}
	if saves == nil {
		return []Save{}
	}
	return saves
}

func listBackupsForSave(kspPath, saveName string) []Backup {
	backupsDir := filepath.Join(kspPath, "saves", "backups")
	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		return []Backup{}
	}
	prefix := saveName + "_"
	var backups []Backup
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) || !strings.HasSuffix(e.Name(), ".zip") {
			continue
		}
		var sizeMB float64
		var created string
		if info, err := e.Info(); err == nil {
			sizeMB = toMB(info.Size())
			created = info.ModTime().Format("2006-01-02 15:04:05")
		} else {
			created = "Unknown"
		}
		backups = append(backups, Backup{Filename: e.Name(), SizeMB: sizeMB, Created: created})
	}
	if backups == nil {
		return []Backup{}
	}
	return backups
}

// BackupResult is returned by BackupSave on success.
type BackupResult struct {
	Filename string  `json:"filename"`
	Path     string  `json:"path"`
	SizeMB   float64 `json:"size_mb"`
}

// BackupSave creates a timestamped zip backup of the named save.
// Returns nil, nil when the save directory does not exist.
func BackupSave(kspPath, saveName string) (*BackupResult, error) {
	savePath := filepath.Join(kspPath, "saves", saveName)
	if _, err := os.Stat(savePath); err != nil {
		return nil, nil
	}

	backupsDir := filepath.Join(kspPath, "saves", "backups")
	if err := os.MkdirAll(backupsDir, 0755); err != nil {
		return nil, err
	}

	timestamp := time.Now().Format("20060102_150405")
	zipName := fmt.Sprintf("%s_%s.zip", saveName, timestamp)
	zipPath := filepath.Join(backupsDir, zipName)

	if err := createSaveZip(savePath, zipPath, saveName); err != nil {
		return nil, err
	}

	info, err := os.Stat(zipPath)
	if err != nil {
		return nil, err
	}
	return &BackupResult{Filename: zipName, Path: zipPath, SizeMB: toMB(info.Size())}, nil
}

func createSaveZip(srcPath, zipPath, arcPrefix string) error {
	zf, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zf.Close()

	zw := zip.NewWriter(zf)
	defer zw.Close()

	return filepath.Walk(srcPath, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(srcPath, p)
		arcName := filepath.ToSlash(filepath.Join(arcPrefix, rel))
		w, err := zw.Create(arcName)
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
}
