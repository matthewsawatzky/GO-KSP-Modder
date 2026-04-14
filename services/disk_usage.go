package services

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// folderSize recursively sums file sizes under path (errors skipped).
func folderSize(path string) int64 {
	var total int64
	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

// toMB converts bytes to MB rounded to 2 decimal places.
func toMB(bytes int64) float64 {
	return math.Round(float64(bytes)/1024/1024*100) / 100
}

// DiskUsage maps folder names to their sizes in MB.
type DiskUsage map[string]float64

// GetDiskUsage returns disk usage for key KSP directories plus total.
func GetDiskUsage(kspPath string) DiskUsage {
	usage := make(DiskUsage)
	for _, name := range []string{"GameData", "saves", "Screenshots", "Ships"} {
		usage[name] = toMB(folderSize(filepath.Join(kspPath, name)))
	}
	usage["Total"] = toMB(folderSize(kspPath))
	return usage
}

// GetGameVersion tries to read the game version from buildID or readme files.
func GetGameVersion(kspPath string) string {
	for _, fname := range []string{"buildID.txt", "buildID64.txt"} {
		f, err := os.Open(filepath.Join(kspPath, fname))
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(strings.ToLower(line), "build id =") {
				f.Close()
				if parts := strings.SplitN(line, "=", 2); len(parts) == 2 {
					return strings.TrimSpace(parts[1])
				}
			}
		}
		f.Close()
	}
	if f, err := os.Open(filepath.Join(kspPath, "readme.txt")); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		if scanner.Scan() {
			return strings.TrimSpace(scanner.Text())
		}
	}
	return "Unknown"
}

// Screenshot represents a screenshot file.
type Screenshot struct {
	Filename string  `json:"filename"`
	SizeMB   float64 `json:"size_mb"`
}

// ListScreenshots returns image files from the Screenshots directory.
func ListScreenshots(kspPath string) []Screenshot {
	ssDir := filepath.Join(kspPath, "Screenshots")
	entries, err := os.ReadDir(ssDir)
	if err != nil {
		return []Screenshot{}
	}
	valid := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".bmp": true, ".tga": true}
	var out []Screenshot
	for _, e := range entries {
		if e.IsDir() || !valid[strings.ToLower(filepath.Ext(e.Name()))] {
			continue
		}
		var sizeMB float64
		if info, err := e.Info(); err == nil {
			sizeMB = toMB(info.Size())
		}
		out = append(out, Screenshot{Filename: e.Name(), SizeMB: sizeMB})
	}
	if out == nil {
		return []Screenshot{}
	}
	return out
}

// Craft represents a craft design file.
type Craft struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	SizeKB   float64 `json:"size_kb"`
	Modified string  `json:"modified"`
}

// ListCrafts returns craft files from Ships/VAB and Ships/SPH.
func ListCrafts(kspPath string) []Craft {
	var crafts []Craft
	shipsDir := filepath.Join(kspPath, "Ships")
	for _, craftType := range []string{"VAB", "SPH"} {
		entries, err := os.ReadDir(filepath.Join(shipsDir, craftType))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".craft") {
				continue
			}
			var sizeKB float64
			var modified string
			if info, err := e.Info(); err == nil {
				sizeKB = math.Round(float64(info.Size())/1024*10) / 10
				modified = info.ModTime().Format("2006-01-02 15:04:05")
			} else {
				modified = "Unknown"
			}
			crafts = append(crafts, Craft{
				Name:     strings.TrimSuffix(e.Name(), ".craft"),
				Type:     craftType,
				SizeKB:   sizeKB,
				Modified: modified,
			})
		}
	}
	if crafts == nil {
		return []Craft{}
	}
	return crafts
}
