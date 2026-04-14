package services

import (
	"os"
	"path/filepath"
	"runtime"
)

// Install represents a detected KSP installation.
type Install struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

func getCandidatePaths() []Install {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return []Install{
			{"Steam (KSP1)", filepath.Join(home, "Library", "Application Support", "Steam", "steamapps", "common", "Kerbal Space Program")},
			{"GOG (KSP1) - User Apps", filepath.Join(home, "Applications", "Kerbal Space Program")},
			{"GOG (KSP1) - System Apps", "/Applications/Kerbal Space Program"},
			{"Steam (KSP2)", filepath.Join(home, "Library", "Application Support", "Steam", "steamapps", "common", "Kerbal Space Program 2")},
		}
	case "windows":
		return []Install{
			{"Steam (KSP1) - Program Files x86", `C:\Program Files (x86)\Steam\steamapps\common\Kerbal Space Program`},
			{"Steam (KSP1) - Program Files", `C:\Program Files\Steam\steamapps\common\Kerbal Space Program`},
			{"GOG (KSP1)", `C:\GOG Games\Kerbal Space Program`},
			{"Epic (KSP1)", `C:\Program Files\Epic Games\KerbalSpaceProgram`},
			{"Steam (KSP2) - Program Files x86", `C:\Program Files (x86)\Steam\steamapps\common\Kerbal Space Program 2`},
			{"Steam (KSP2) - Program Files", `C:\Program Files\Steam\steamapps\common\Kerbal Space Program 2`},
			{"Epic (KSP2)", `C:\Program Files\Epic Games\KerbalSpaceProgram2`},
		}
	}
	return nil
}

// ValidateKSPInstall returns true if path looks like a valid KSP install directory.
func ValidateKSPInstall(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	for _, exe := range []string{"KSP.app", "KSP_x64.app", "KSP.exe", "KSP_x64.exe", "KSP2.app", "KSP2.exe"} {
		if _, err := os.Stat(filepath.Join(path, exe)); err == nil {
			return true
		}
	}
	if info, err := os.Stat(filepath.Join(path, "GameData")); err == nil && info.IsDir() {
		return true
	}
	return false
}

// DetectInstalls scans standard installation locations and returns found installs.
func DetectInstalls() []Install {
	var found []Install
	for _, c := range getCandidatePaths() {
		if ValidateKSPInstall(c.Path) {
			found = append(found, c)
		}
	}
	if found == nil {
		return []Install{}
	}
	return found
}
