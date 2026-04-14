package services

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func findLogFile(kspPath string) string {
	candidates := []string{
		filepath.Join(kspPath, "KSP.log"),
		filepath.Join(kspPath, "KSP2.log"),
	}
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		candidates = append(candidates, filepath.Join(home, "Library", "Logs", "Unity", "Player.log"))
	case "windows":
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			local := filepath.Join(filepath.Dir(appdata), "LocalLow", "Squad", "Kerbal Space Program", "Player.log")
			candidates = append(candidates, local)
		}
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func isErrorLine(line string) bool {
	for _, m := range []string{"[ERR]", "[EXC]", "Exception", "Error"} {
		if strings.Contains(line, m) {
			return true
		}
	}
	return false
}

func isWarningLine(line string) bool {
	for _, m := range []string{"[WRN]", "Warning"} {
		if strings.Contains(line, m) {
			return true
		}
	}
	return false
}

// readAllLines reads every line from path, ignoring decode errors.
func readAllLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	// Allow up to 1 MB per line to avoid scanner token-too-long errors.
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	// scanner.Err() intentionally ignored to match Python errors='replace' behaviour.
	return lines, nil
}

// LogResult is the response returned by ReadLog.
type LogResult struct {
	Lines      []string `json:"lines"`
	Path       *string  `json:"path"`
	TotalLines int      `json:"total_lines"`
	Error      *string  `json:"error"`
}

// ReadLog reads the game log and filters lines according to filterMode.
// filterMode must be one of: "all", "errors", "warnings", "errors+warnings".
func ReadLog(kspPath, filterMode string) LogResult {
	logPath := findLogFile(kspPath)
	if logPath == "" {
		msg := "No log file found"
		return LogResult{Lines: []string{}, Path: nil, Error: &msg}
	}

	all, err := readAllLines(logPath)
	if err != nil {
		msg := err.Error()
		return LogResult{Lines: []string{}, Path: &logPath, Error: &msg}
	}

	var filtered []string
	switch filterMode {
	case "errors":
		for _, l := range all {
			if isErrorLine(l) {
				filtered = append(filtered, l)
			}
		}
	case "warnings":
		for _, l := range all {
			if isWarningLine(l) {
				filtered = append(filtered, l)
			}
		}
	case "errors+warnings":
		for _, l := range all {
			if isErrorLine(l) || isWarningLine(l) {
				filtered = append(filtered, l)
			}
		}
	default:
		filtered = all
	}

	if len(filtered) > 500 {
		filtered = filtered[len(filtered)-500:]
	}
	if filtered == nil {
		filtered = []string{}
	}
	return LogResult{Lines: filtered, Path: &logPath, TotalLines: len(all), Error: nil}
}

// ModErrorEntry holds error lines attributed to a single mod.
type ModErrorEntry struct {
	Lines []string `json:"lines"`
	Total int      `json:"total"`
}

// ModErrorResult is the response returned by ScanModErrors.
type ModErrorResult struct {
	Error             *string                  `json:"error"`
	Results           map[string]ModErrorEntry `json:"results"`
	UnattributedCount int                      `json:"unattributed_count"`
	TotalErrors       int                      `json:"total_errors"`
}

// ScanModErrors scans the log and groups error lines by which mod they mention.
func ScanModErrors(kspPath string, modNames []string) ModErrorResult {
	empty := map[string]ModErrorEntry{}

	logPath := findLogFile(kspPath)
	if logPath == "" {
		msg := "No log file found"
		return ModErrorResult{Error: &msg, Results: empty}
	}

	all, err := readAllLines(logPath)
	if err != nil {
		msg := err.Error()
		return ModErrorResult{Error: &msg, Results: empty}
	}

	var errorLines []string
	for _, l := range all {
		if isErrorLine(l) {
			errorLines = append(errorLines, l)
		}
	}

	// Build lowercase → original name map.
	lookup := make(map[string]string, len(modNames))
	for _, name := range modNames {
		if name != "" {
			lookup[strings.ToLower(name)] = name
		}
	}

	const maxPerMod = 20
	results := make(map[string]ModErrorEntry)
	unattributed := 0

	for _, line := range errorLines {
		lower := strings.ToLower(line)
		var matched []string
		for lk, orig := range lookup {
			if strings.Contains(lower, lk) {
				matched = append(matched, orig)
			}
		}
		if len(matched) > 0 {
			for _, mod := range matched {
				e := results[mod]
				e.Total++
				if len(e.Lines) < maxPerMod {
					e.Lines = append(e.Lines, line)
				}
				results[mod] = e
			}
		} else {
			unattributed++
		}
	}

	if len(results) == 0 {
		results = empty
	}
	return ModErrorResult{
		Error:             nil,
		Results:           results,
		UnattributedCount: unattributed,
		TotalErrors:       len(errorLines),
	}
}
