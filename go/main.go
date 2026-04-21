package main

import (
	"bufio"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"ksp-moder/config"
	"ksp-moder/handlers"
)

//go:embed static
var staticFiles embed.FS

// ── ANSI colour codes ─────────────────────────────────────────────────────────

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiGreen  = "\033[38;2;138;192;74m" // KSP brand green #8AC04A
	ansiCyan   = "\033[36m"
	ansiWhite  = "\033[97m"
	ansiYellow = "\033[33m"
	ansiRed    = "\033[31m"
)

// ── ASCII art ─────────────────────────────────────────────────────────────────

var kspArt = []string{
	`  ██╗  ██╗███████╗██████╗ `,
	`  ██║ ██╔╝██╔════╝██╔══██╗`,
	`  █████╔╝ ███████╗██████╔╝`,
	`  ██╔═██╗ ╚════██║██╔═══╝ `,
	`  ██║  ██╗███████║██║     `,
	`  ╚═╝  ╚═╝╚══════╝╚═╝     `,
}

const (
	starfield = "  ·   *   ·    *    ·   *    ·    *   ·    *   ·    *   ·"
	separator = "  ────────────────────────────────────────────────────────"
)

// ── Death screens ─────────────────────────────────────────────────────────────
//
// Each entry is shown when this instance is remotely closed by a new one.
// Replace the art inside any block with your own — keep lines under ~56 chars.
// A random screen is picked each time.

var deathScreens = [][]string{

	// ── TEMPLATE 1 — Tombstone ───────────────────────────────────────────────
	{
		`            .=======.          `,
		`           ||       ||         `,
		`           ||  RIP  ||         `,
		`           ||       ||         `,
		`           ||  KSP  ||         `,
		`           || Moder ||         `,
		`           |'======='|         `,
		`            )       (          `,
		`           (__________)        `,
		`   _____________________________`,
		`  /~~~~~~~~~~~~~~~~~~~~~~~~~~~~~\`,
	},

	// ── TEMPLATE 2 — Rapid Unplanned Disassembly ─────────────────────────────
	{
		`        *    .   *   .    *     `,
		`      .   \ *  .   .  * /   .  `,
		`     *  *  \    * *    /  *  * `,
		`    . --*---[  R·U·D  ]---*-- .`,
		`     *  *  /    * *    \  *  * `,
		`      .   / *  .   .  * \   .  `,
		`        *    .   *   .    *     `,
		`                               `,
		`    Rapid Unplanned            `,
		`    Disassembly complete.      `,
	},

	// ── TEMPLATE 3 — Kerbal adrift ───────────────────────────────────────────
	{
		`              O ))             `,
		`             \|               `,
		`             / \              `,
		`                              `,
		`    *    .       *    .    *  `,
		`       *    .  *    .         `,
		`    .    *    .    *    .     `,
		`       *    .    *    .    *  `,
		`                              `,
		`    This Kerbal is gone.      `,
		`    Press F to pay respects.  `,
	},
}

// ── Fun kill messages ─────────────────────────────────────────────────────────

var killMessages = []string{
	"Target eliminated. Port is yours, Commander.",
	"Other instance nuked from orbit. It's the only way to be sure.",
	"Another Kerbal bites the dust.",
	"Ker-BOOM. Port secured.",
	"The other window didn't have enough delta-v.",
	"Mission control has lost contact with the other instance.",
	"404: Other instance not found (anymore).",
	"Jeb hit the big red button. Port is clear.",
	"RIP other instance. You will not be missed.",
	"Instance terminated. The Kraken is satisfied.",
	"Other instance deorbited successfully.",
	"Disassembly complete. Proceed with launch.",
}

// ── Spinner ───────────────────────────────────────────────────────────────────

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ── Remote-quit signal ────────────────────────────────────────────────────────

// remoteQuitCh is signalled by the /api/quit handler so the key loop can
// restore the terminal before printing the death screen and exiting.
var remoteQuitCh = make(chan struct{}, 1)

// ── Screen helpers ────────────────────────────────────────────────────────────

func clearScreen() { fmt.Print("\033[2J\033[H") }

func printLogo() {
	for i, line := range kspArt {
		switch i {
		case 1:
			fmt.Printf("%s%s%s  %sMOD MANAGER%s\n",
				ansiGreen, line, ansiReset, ansiBold+ansiWhite, ansiReset)
		case 2:
			fmt.Printf("%s%s%s  %sKerbal Space Program%s\n",
				ansiGreen, line, ansiReset, ansiDim, ansiReset)
		default:
			fmt.Printf("%s%s%s\n", ansiGreen, line, ansiReset)
		}
	}
}

func printStartupScreen() {
	clearScreen()
	fmt.Println()
	fmt.Println(ansiDim + starfield + ansiReset)
	fmt.Println()
	printLogo()
	fmt.Println()
	fmt.Println(ansiDim + separator + ansiReset)
}

func printBanner(port int) {
	url := fmt.Sprintf("http://localhost:%d", port)
	clearScreen()
	fmt.Println()
	fmt.Println(ansiDim + starfield + ansiReset)
	fmt.Println()
	printLogo()
	fmt.Println()
	fmt.Println(ansiDim + separator + ansiReset)
	fmt.Printf("  Server  →  %s%s%s\n", ansiBold+ansiCyan, url, ansiReset)
	fmt.Println(ansiDim + separator + ansiReset)
	fmt.Println()
	fmt.Printf("  %s[O]%s Open in browser      %s[Q]%s Quit\n",
		ansiBold+ansiGreen, ansiReset,
		ansiBold+ansiGreen, ansiReset)
	fmt.Println()
}

// showDeathScreen clears the terminal and prints a random death-screen art.
// Call this only after the terminal has been restored from raw mode.
func showDeathScreen() {
	art := deathScreens[rand.Intn(len(deathScreens))]
	clearScreen()
	fmt.Println()
	fmt.Println(ansiDim + starfield + ansiReset)
	fmt.Println()
	for _, line := range art {
		fmt.Println(ansiGreen + "  " + line + ansiReset)
	}
	fmt.Println()
	fmt.Println(ansiDim + separator + ansiReset)
	fmt.Printf("  %sThis session was closed by a new instance.%s\n", ansiBold+ansiRed, ansiReset)
	fmt.Println(ansiDim + separator + ansiReset)
	fmt.Println()
}

// typewriterPrint prints s character-by-character with a short delay.
func typewriterPrint(s, color string) {
	fmt.Print("  " + color)
	for _, ch := range s {
		fmt.Printf("%c", ch)
		time.Sleep(35 * time.Millisecond)
	}
	fmt.Println(ansiReset)
}

// ── Instance detection ────────────────────────────────────────────────────────

func isOurInstance(port int) bool {
	client := &http.Client{Timeout: 600 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/isprogramopen", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}
	return strings.Contains(string(body), "ksp-moder")
}

func quitOtherInstance(port int) error {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post(
		fmt.Sprintf("http://127.0.0.1:%d/api/quit", port),
		"application/json", nil,
	)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// waitForPortFree spins with a braille spinner until the port clears or 5 s pass.
func waitForPortFree(port int) bool {
	deadline := time.Now().Add(5 * time.Second)
	i := 0
	for time.Now().Before(deadline) {
		if !isPortInUse(port) {
			fmt.Print("\r\033[K")
			return true
		}
		fmt.Printf("\r  %s%s%s  Clearing port...",
			ansiGreen, spinnerFrames[i%len(spinnerFrames)], ansiReset)
		time.Sleep(100 * time.Millisecond)
		i++
	}
	fmt.Print("\r\033[K")
	return false
}

// showConflictScreen renders the conflict UI and returns "takeover" or "quit".
func showConflictScreen(port int) string {
	clearScreen()
	fmt.Println()
	fmt.Println(ansiDim + starfield + ansiReset)
	fmt.Println()
	printLogo()
	fmt.Println()
	fmt.Printf("  %s! Another KSP Moder is already running on port %d%s\n",
		ansiYellow, port, ansiReset)
	fmt.Println(ansiDim + separator + ansiReset)
	fmt.Println()
	fmt.Printf("  %s[O]%s  Take over — quit the other instance\n",
		ansiBold+ansiGreen, ansiReset)
	fmt.Printf("  %s[Q]%s  Stand down — quit this one\n",
		ansiBold+ansiGreen, ansiReset)
	fmt.Println()

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("  Choice [O/Q]: ")
		input, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(input)) == "q" {
			return "quit"
		}
		return "takeover"
	}

	buf := make([]byte, 1)
	for {
		n, readErr := os.Stdin.Read(buf)
		if readErr != nil || n == 0 {
			term.Restore(int(os.Stdin.Fd()), oldState) //nolint:errcheck
			return "quit"
		}
		switch buf[0] {
		case 'o', 'O':
			term.Restore(int(os.Stdin.Fd()), oldState) //nolint:errcheck
			return "takeover"
		case 'q', 'Q', 3:
			term.Restore(int(os.Stdin.Fd()), oldState) //nolint:errcheck
			return "quit"
		}
	}
}

// ── Port prompt ───────────────────────────────────────────────────────────────

func promptPort() int {
	const defaultPort = 5050
	printStartupScreen()

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("  Enter port %s[%d]%s: ", ansiDim, defaultPort, ansiReset)
		input, err := reader.ReadString('\n')
		if err != nil {
			return defaultPort
		}
		input = strings.TrimSpace(input)

		port := defaultPort
		if input != "" {
			p, convErr := strconv.Atoi(input)
			if convErr != nil || p < 1 || p > 65535 {
				fmt.Printf("  %sInvalid port.%s Enter a number between 1 and 65535.\n",
					ansiYellow, ansiReset)
				continue
			}
			port = p
		}

		if isPortInUse(port) {
			if isOurInstance(port) {
				choice := showConflictScreen(port)
				if choice == "quit" {
					fmt.Print("\r\n" + ansiDim + "  Standing down..." + ansiReset + "\r\n")
					os.Exit(0)
				}
				if err := quitOtherInstance(port); err != nil {
					printStartupScreen()
					fmt.Printf("  %sCould not reach the other instance: %v%s\n",
						ansiYellow, err, ansiReset)
					continue
				}
				if !waitForPortFree(port) {
					printStartupScreen()
					fmt.Printf("  %sPort %d did not clear in time. Try a different port.%s\n",
						ansiYellow, port, ansiReset)
					continue
				}
				typewriterPrint(killMessages[rand.Intn(len(killMessages))], ansiGreen)
				time.Sleep(700 * time.Millisecond)
				return port
			}
			fmt.Printf("  %sPort %d is already in use by another program.%s Please choose another.\n",
				ansiYellow, port, ansiReset)
			continue
		}

		return port
	}
}

func isPortInUse(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return true
	}
	ln.Close()
	return false
}

// ── Key loop ──────────────────────────────────────────────────────────────────

func runKeyLoop(url string) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	oldState, rawErr := term.MakeRaw(int(os.Stdin.Fd()))
	if rawErr != nil {
		select {
		case <-sigCh:
		case <-remoteQuitCh:
			showDeathScreen()
		}
		fmt.Println("\nStopping server...")
		os.Exit(0)
	}

	restore := func() {
		term.Restore(int(os.Stdin.Fd()), oldState) //nolint:errcheck
	}

	// Handle OS signals and remote-quit in a dedicated goroutine.
	go func() {
		select {
		case <-sigCh:
			restore()
			fmt.Print("\r\n" + ansiDim + "  Stopping server..." + ansiReset + "\r\n")
			os.Exit(0)
		case <-remoteQuitCh:
			restore()
			showDeathScreen()
			os.Exit(0)
		}
	}()

	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil || n == 0 {
			break
		}
		switch buf[0] {
		case 'o', 'O':
			openBrowser(url)
		case 'q', 'Q', 3:
			restore()
			fmt.Print("\r\n" + ansiDim + "  Stopping server..." + ansiReset + "\r\n")
			os.Exit(0)
		}
	}
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	configPath, err := config.DefaultConfigPath()
	if err != nil {
		log.Fatalf("could not determine config directory: %v", err)
	}

	migrateConfig(configPath)

	cfg := config.NewManager(configPath)
	if err := cfg.EnsureConfig(); err != nil {
		log.Fatalf("failed to initialise config: %v", err)
	}

	port := promptPort()
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	url := fmt.Sprintf("http://localhost:%d", port)

	mux := http.NewServeMux()

	// ── Static files ──────────────────────────────────────────────────────────
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		data, err := staticFiles.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "index.html not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data) //nolint:errcheck
	})

	// ── Instance lifecycle endpoints ──────────────────────────────────────────

	// Fingerprint — identifies this process as KSP Moder to new instances.
	mux.HandleFunc("GET /isprogramopen", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"app":"ksp-moder"}`)
	})

	// Remote-quit — only honoured from loopback; signals the key loop to exit.
	mux.HandleFunc("POST /api/quit", func(w http.ResponseWriter, r *http.Request) {
		host, _, splitErr := net.SplitHostPort(r.RemoteAddr)
		if splitErr != nil || (host != "127.0.0.1" && host != "::1") {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"shutting down"}`)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		go func() {
			time.Sleep(150 * time.Millisecond)
			remoteQuitCh <- struct{}{}
		}()
	})

	// ── API routes ────────────────────────────────────────────────────────────
	h := handlers.New(cfg)

	mux.HandleFunc("GET /api/detect-installs", h.DetectInstalls)
	mux.HandleFunc("POST /api/set-path", h.SetPath)
	mux.HandleFunc("GET /api/current-path", h.CurrentPath)
	mux.HandleFunc("GET /api/settings", h.GetSettings)
	mux.HandleFunc("POST /api/settings", h.SaveSettings)

	mux.HandleFunc("GET /api/mods", h.ListMods)
	mux.HandleFunc("POST /api/mods/add", h.AddMod)
	mux.HandleFunc("GET /api/mods/notes", h.GetNotes)
	mux.HandleFunc("POST /api/mods/notes", h.SetNote)
	mux.HandleFunc("POST /api/mods/bulk", h.BulkAction)
	mux.HandleFunc("DELETE /api/mods/{name}", h.RemoveMod)
	mux.HandleFunc("POST /api/mods/{name}/toggle", h.ToggleMod)

	mux.HandleFunc("GET /api/profiles", h.ListProfiles)
	mux.HandleFunc("POST /api/profiles/save", h.SaveProfile)
	mux.HandleFunc("POST /api/profiles/load", h.LoadProfile)
	mux.HandleFunc("DELETE /api/profiles/{name}", h.DeleteProfile)

	mux.HandleFunc("GET /api/saves", h.ListSaves)
	mux.HandleFunc("POST /api/saves/{name}/backup", h.BackupSave)

	mux.HandleFunc("GET /api/logs", h.GetLogs)
	mux.HandleFunc("GET /api/logs/mod-errors", h.GetModErrors)

	mux.HandleFunc("GET /api/info", h.GetInfo)
	mux.HandleFunc("GET /api/screenshots", h.ListScreenshots)
	mux.HandleFunc("GET /screenshots/{filename}", h.ServeScreenshot)
	mux.HandleFunc("GET /api/crafts", h.ListCrafts)

	// ── Start server ──────────────────────────────────────────────────────────
	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			fmt.Fprintf(os.Stderr, "\r\nServer error: %v\n", err)
			os.Exit(1)
		}
	}()

	go func() {
		time.Sleep(time.Second)
		openBrowser(url)
	}()

	printBanner(port)
	runKeyLoop(url)
}

// ── Config migration ──────────────────────────────────────────────────────────

// migrateConfig silently copies a config.json sitting next to the binary into
// the new platform config directory, if the new location doesn't exist yet.
func migrateConfig(newPath string) {
	if _, err := os.Stat(newPath); err == nil {
		return // new location already exists — nothing to do
	}
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	oldPath := filepath.Join(filepath.Dir(exePath), "config.json")
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return // no old config either — fresh install
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		return
	}
	os.WriteFile(newPath, data, 0644) //nolint:errcheck
}

// ── Browser opener ────────────────────────────────────────────────────────────

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
