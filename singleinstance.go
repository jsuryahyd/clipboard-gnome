package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"clipboard-gnome/internal/logger"
)

type InstanceRequest struct {
	Action  string `json:"action"` // "status", "show", "quit"
	Version string `json:"version,omitempty"`
	PID     int    `json:"pid,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type InstanceResponse struct {
	Status     string `json:"status"` // "ok", "quitting", "error"
	Version    string `json:"version,omitempty"`
	BuildDate  string `json:"build_date,omitempty"`
	PID        int    `json:"pid,omitempty"`
	StartTime  int64  `json:"start_time,omitempty"`
	Error      string `json:"error,omitempty"`
}

type SingleInstanceManager struct {
	sockPath    string
	listener    net.Listener
	app         *App
	StartHidden bool
	mu          sync.Mutex
	closed      bool
	startTime   int64
}

// InitSingleInstance checks command-line arguments and manages single-instance behavior.
// If another instance is running and up-to-date, it signals the running instance and exits the process.
// If the current instance is newer or --replace is given, it instructs the old instance to terminate,
// waits for it to exit, and becomes the active instance.
func InitSingleInstance(app *App, args []string) *SingleInstanceManager {
	var startHidden bool
	var forceReplace bool

	for _, arg := range args {
		// Ignore bindings generation from Wails
		if strings.Contains(arg, "bindings") {
			return &SingleInstanceManager{StartHidden: true}
		}

		switch arg {
		case "-v", "--version":
			fmt.Printf("clipboard-gnome version %s\n", Version)
			os.Exit(0)
		case "-h", "--help":
			fmt.Println("Clipboard-Gnome: Hybrid Wayland Clipboard Manager")
			fmt.Println("\nUsage:")
			fmt.Println("  clipboard-gnome [flags]")
			fmt.Println("\nFlags:")
			fmt.Println("  --hidden       Start application hidden in background")
			fmt.Println("  --replace      Replace any currently running instance")
			fmt.Println("  -v, --version  Print application version and exit")
			fmt.Println("  -h, --help     Show this help message")
			os.Exit(0)
		case "--hidden":
			startHidden = true
		case "--replace":
			forceReplace = true
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	stateDir := filepath.Join(homeDir, ".local", "state", "clipboard-gnome")
	_ = os.MkdirAll(stateDir, 0755)
	sockPath := filepath.Join(stateDir, "clipboard-gnome.sock")

	mgr := &SingleInstanceManager{
		sockPath:    sockPath,
		app:         app,
		StartHidden: startHidden,
		startTime:   time.Now().Unix(),
	}

	// Try connecting to existing socket
	conn, err := net.DialTimeout("unix", sockPath, 500*time.Millisecond)
	if err == nil {
		// Existing instance is listening on socket
		defer conn.Close()

		// Request status
		statusReq := InstanceRequest{Action: "status"}
		data, _ := json.Marshal(statusReq)
		_, _ = conn.Write(append(data, '\n'))

		reader := bufio.NewReader(conn)
		line, readErr := reader.ReadBytes('\n')

		var resp InstanceResponse
		if readErr == nil {
			_ = json.Unmarshal(line, &resp)
		}

		if resp.PID > 0 {
			cmp := CompareVersions(Version, resp.Version)
			isNewer := cmp > 0 || forceReplace

			if cmp == 0 && !isNewer {
				if BuildDate != "" && resp.BuildDate != "" && BuildDate > resp.BuildDate {
					isNewer = true
				} else {
					// Check binary file modification time against running process start time
					execPath, execErr := os.Executable()
					if execErr == nil {
						if fi, statErr := os.Stat(execPath); statErr == nil {
							if procFi, procErr := os.Stat(fmt.Sprintf("/proc/%d", resp.PID)); procErr == nil {
								if fi.ModTime().After(procFi.ModTime()) {
									isNewer = true
								}
							}
						}
					}
				}
			}

			if isNewer {
				fmt.Printf("Newer version or binary detected (%s vs running %s, PID %d). Replacing running instance...\n",
					Version, resp.Version, resp.PID)

				quitReq := InstanceRequest{
					Action:  "quit",
					Version: Version,
					PID:     os.Getpid(),
					Reason:  "upgrade",
				}
				qData, _ := json.Marshal(quitReq)
				_, _ = conn.Write(append(qData, '\n'))
				_, _ = reader.ReadBytes('\n')

				// Wait for running process to terminate
				waitForProcessExit(resp.PID, 2*time.Second)
				_ = os.Remove(sockPath)
			} else {
				// Current binary is not newer; signal the running instance to show UI
				showReq := InstanceRequest{
					Action:  "show",
					Version: Version,
					PID:     os.Getpid(),
				}
				sData, _ := json.Marshal(showReq)
				_, _ = conn.Write(append(sData, '\n'))
				_, _ = reader.ReadBytes('\n')

				fmt.Printf("Clipboard-Gnome is already running (PID %d, version %s). Sent show command.\n",
					resp.PID, resp.Version)
				os.Exit(0)
			}
		} else {
			// Socket was responding but returned invalid status; remove stale socket
			_ = os.Remove(sockPath)
		}
	} else {
		// Connection failed; clean up any stale socket file
		_ = os.Remove(sockPath)
	}

	// Terminate any legacy un-socketed clipboard-gnome processes
	killLegacyInstances(os.Getpid())

	// Start listening on socket
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		fmt.Printf("Warning: Failed to create single-instance socket: %v\n", err)
	} else {
		_ = os.Chmod(sockPath, 0600)
		mgr.listener = listener
		go mgr.serveSocket()
	}

	return mgr
}

func (m *SingleInstanceManager) serveSocket() {
	for {
		conn, err := m.listener.Accept()
		if err != nil {
			m.mu.Lock()
			isClosed := m.closed
			m.mu.Unlock()
			if isClosed {
				return
			}
			continue
		}

		go m.handleConnection(conn)
	}
}

func (m *SingleInstanceManager) handleConnection(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}

		var req InstanceRequest
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}

		switch req.Action {
		case "status":
			resp := InstanceResponse{
				Status:    "ok",
				Version:   Version,
				BuildDate: BuildDate,
				PID:       os.Getpid(),
				StartTime: m.startTime,
			}
			data, _ := json.Marshal(resp)
			_, _ = conn.Write(append(data, '\n'))

		case "show":
			resp := InstanceResponse{Status: "ok"}
			data, _ := json.Marshal(resp)
			_, _ = conn.Write(append(data, '\n'))

			if m.app != nil {
				logger.Info("SingleInstance: Received show request from PID %d", req.PID)
				m.app.onShowUI()
			}

		case "quit":
			logger.Info("SingleInstance: Received quit request from newer version (%s, PID %d). Shutting down...",
				req.Version, req.PID)
			resp := InstanceResponse{Status: "quitting"}
			data, _ := json.Marshal(resp)
			_, _ = conn.Write(append(data, '\n'))

			m.Close()
			if m.app != nil && m.app.ctx != nil {
				m.app.shutdown(m.app.ctx)
			}
			os.Exit(0)
		}
	}
}

func (m *SingleInstanceManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return
	}
	m.closed = true

	if m.listener != nil {
		_ = m.listener.Close()
	}
	_ = os.Remove(m.sockPath)
}

func waitForProcessExit(pid int, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if err != nil {
			// Process does not exist anymore
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	// Force kill if still alive
	_ = syscall.Kill(pid, syscall.SIGKILL)
	time.Sleep(50 * time.Millisecond)
}

func killLegacyInstances(currentPID int) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == currentPID {
			continue
		}

		// Check the actual executable target via /proc/<pid>/exe symlink
		exePath, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
		if err != nil {
			// May fail for permissions or zombie processes; fallback to comm
			commBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
			if err != nil {
				continue
			}
			comm := strings.TrimSpace(string(commBytes))
			if comm != "clipboard-gnome" {
				continue
			}
		} else {
			if filepath.Base(exePath) != "clipboard-gnome" {
				continue
			}
		}

		logger.Info("Terminating legacy/duplicate clipboard-gnome process (PID %d)...", pid)
		_ = syscall.Kill(pid, syscall.SIGTERM)
		time.Sleep(50 * time.Millisecond)
		if syscall.Kill(pid, 0) == nil {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	}
}
