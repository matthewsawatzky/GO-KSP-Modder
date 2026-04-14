package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/term"

	"ksp-moder/config"
	"ksp-moder/handlers"
)

//go:embed static
var staticFiles embed.FS

func main() {
	// Store config.json next to the binary executable.
	exePath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}
	exeDir := filepath.Dir(exePath)
	configPath := filepath.Join(exeDir, "config.json")

	cfg := config.NewManager(configPath)
	if err := cfg.EnsureConfig(); err != nil {
		log.Fatalf("failed to initialise config: %v", err)
	}

	mux := http.NewServeMux()

	// ── Static files ──────────────────────────────────────────────────────────
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Root → index.html
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		data, err := staticFiles.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data) //nolint:errcheck
	})

	// ── API routes ────────────────────────────────────────────────────────────
	h := handlers.New(cfg)

	// Setup
	mux.HandleFunc("GET /api/detect-installs", h.DetectInstalls)
	mux.HandleFunc("POST /api/set-path", h.SetPath)
	mux.HandleFunc("GET /api/current-path", h.CurrentPath)
	mux.HandleFunc("GET /api/settings", h.GetSettings)
	mux.HandleFunc("POST /api/settings", h.SaveSettings)

	// Mods — specific patterns before wildcard ones
	mux.HandleFunc("GET /api/mods", h.ListMods)
	mux.HandleFunc("POST /api/mods/add", h.AddMod)
	mux.HandleFunc("GET /api/mods/notes", h.GetNotes)
	mux.HandleFunc("POST /api/mods/notes", h.SetNote)
	mux.HandleFunc("POST /api/mods/bulk", h.BulkAction)
	mux.HandleFunc("DELETE /api/mods/{name}", h.RemoveMod)
	mux.HandleFunc("POST /api/mods/{name}/toggle", h.ToggleMod)

	// Profiles
	mux.HandleFunc("GET /api/profiles", h.ListProfiles)
	mux.HandleFunc("POST /api/profiles/save", h.SaveProfile)
	mux.HandleFunc("POST /api/profiles/load", h.LoadProfile)
	mux.HandleFunc("DELETE /api/profiles/{name}", h.DeleteProfile)

	// Saves
	mux.HandleFunc("GET /api/saves", h.ListSaves)
	mux.HandleFunc("POST /api/saves/{name}/backup", h.BackupSave)

	// Logs
	mux.HandleFunc("GET /api/logs", h.GetLogs)
	mux.HandleFunc("GET /api/logs/mod-errors", h.GetModErrors)

	// Info / screenshots / crafts
	mux.HandleFunc("GET /api/info", h.GetInfo)
	mux.HandleFunc("GET /api/screenshots", h.ListScreenshots)
	mux.HandleFunc("GET /screenshots/{filename}", h.ServeScreenshot)
	mux.HandleFunc("GET /api/crafts", h.ListCrafts)

	// Open browser after a short delay so the server is ready.
	go func() {
		time.Sleep(time.Second)
		openBrowser("http://localhost:5050")
	}()

	printBanner()
	if err := http.ListenAndServe("0.0.0.0:5050", mux); err != nil {
		log.Fatal(err)
	}
}

func printBanner() {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width < 20 {
		width = 60 // fallback
	}

	inner := width - 2 // subtract the two border chars

	border := "╔" + strings.Repeat("═", inner) + "╗"
	blank := "║" + strings.Repeat(" ", inner) + "║"
	bottom := "╚" + strings.Repeat("═", inner) + "╝"

	center := func(s string, extraWidth int) string {
		// extraWidth for chars that are wide (e.g. emoji = 2 cols but len = 4 bytes)
		pad := inner - len(s) + extraWidth
		left := pad / 2
		right := pad - left
		return "║" + strings.Repeat(" ", left) + s + strings.Repeat(" ", right) + "║"
	}

	fmt.Print("\033[2J\033[H")
	fmt.Println(border)
	fmt.Println(blank)
	fmt.Println(center("🚀 KSP Moder running at", 2)) // 1 = emoji extra width
	fmt.Println(center("http://localhost:5050", 0))
	fmt.Println(blank)
	fmt.Println(center("Press Ctrl+C in terminal to stop the server", 0))
	fmt.Println(center("Or just close the window", 0))
	fmt.Println(blank)
	fmt.Println(bottom)
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{url}
	case "windows":
		cmd, args = "cmd", []string{"/c", "start", url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	exec.Command(cmd, args...).Start() //nolint:errcheck
}
