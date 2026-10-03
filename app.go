package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"clipboard-gnome/internal/db"
	"clipboard-gnome/internal/dbus"
	"clipboard-gnome/internal/logger"
	"clipboard-gnome/internal/settings"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx         context.Context
	database    *db.DB
	dbusClient  *dbus.Client
	incognito   bool
	startHidden bool
	mu          sync.RWMutex
}

func NewApp() *App {
	return &App{
		incognito: false,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	stateDir := filepath.Join(homeDir, ".local", "state", "clipboard-gnome")
	logPath := filepath.Join(stateDir, "app.log")
	dbPath := filepath.Join(stateDir, "clipboard.db")

	// 1. Initialize Structured Logger
	_, err = logger.InitLogger(logPath, logger.INFO)
	if err != nil {
		fmt.Printf("Error initializing logger: %v\n", err)
	}
	logger.Info("Starting Clipboard-Gnome Wails Core Engine...")

	// 2. Initialize SQLite Database
	database, err := db.InitDB(dbPath)
	if err != nil {
		logger.Error("Failed to initialize database: %v", err)
	} else {
		a.database = database
		logger.Info("SQLite Database initialized at %s", dbPath)
	}

	// 3. Connect to DBus Client
	dbusClient, err := dbus.NewClient(a.onClipboardChanged, a.onShowUI)
	if err != nil {
		logger.Warn("DBus Client initialization failed (GNOME Extension might be inactive): %v", err)
	} else {
		a.dbusClient = dbusClient
	}

	// 4. Register Autostart Desktop Entry
	a.ensureAutostart(homeDir)

	// 5. Run initial DB cleanup & periodic maintenance goroutine
	go a.startMaintenanceLoop()
}

func (a *App) shutdown(ctx context.Context) {
	logger.Info("Shutting down Clipboard-Gnome backend...")
	if a.dbusClient != nil {
		a.dbusClient.Close()
	}
	if a.database != nil {
		_ = a.database.Close()
	}
}

func (a *App) onClipboardChanged(itemType, content string) {
	a.mu.RLock()
	isIncognito := a.incognito
	a.mu.RUnlock()

	if isIncognito {
		logger.Info("Incognito mode active: Clipboard change ignored")
		return
	}

	if a.database == nil {
		return
	}

	newItem, err := a.database.InsertItem(itemType, content, "System")
	if err != nil {
		logger.Error("Failed to insert clipboard item: %v", err)
		return
	}

	logger.Info("Recorded new clipboard item (ID: %d, Type: %s)", newItem.ID, newItem.Type)
	runtime.EventsEmit(a.ctx, "clipboard:changed", newItem)
}

func (a *App) domReady(ctx context.Context) {
	logger.Info("Frontend DOM ready event received")
	if a.startHidden {
		runtime.WindowHide(ctx)
	}
}

func (a *App) onShowUI() {
	logger.Info("ShowUI requested (showing window)")
	if a.ctx != nil {
		runtime.WindowShow(a.ctx)
		runtime.WindowUnminimise(a.ctx)
		runtime.WindowSetAlwaysOnTop(a.ctx, true)
		runtime.WindowCenter(a.ctx)
		runtime.EventsEmit(a.ctx, "ui:focus_search", nil)
	}
	if a.dbusClient != nil {
		go func() {
			time.Sleep(50 * time.Millisecond)
			_ = a.dbusClient.ActivateWindow()
		}()
	}
}

func (a *App) startMaintenanceLoop() {
	s := settings.LoadSettings()
	retentionDays := s.RetentionDays
	maxSizeBytes := int64(s.MaxItemSizeMB * 1024 * 1024)

	if a.database != nil {
		deleted, err := a.database.Cleanup(retentionDays, maxSizeBytes)
		if err != nil {
			logger.Error("DB Cleanup error: %v", err)
		} else {
			logger.Info("Initial DB cleanup purged %d expired items", deleted)
		}
	}

	ticker := time.NewTicker(24 * time.Hour)
	for range ticker.C {
		s = settings.LoadSettings()
		if a.database != nil {
			deleted, err := a.database.Cleanup(s.RetentionDays, int64(s.MaxItemSizeMB*1024*1024))
			if err != nil {
				logger.Error("Periodic DB cleanup error: %v", err)
			} else {
				logger.Info("Periodic DB cleanup purged %d items", deleted)
			}
		}
	}
}

func (a *App) ensureAutostart(homeDir string) {
	autostartDir := filepath.Join(homeDir, ".config", "autostart")
	_ = os.MkdirAll(autostartDir, 0755)

	execPath, err := os.Executable()
	if err != nil {
		execPath = "clipboard-gnome"
	}
	installedPath := filepath.Join(homeDir, ".local", "bin", "clipboard-gnome")
	if _, err := os.Stat(installedPath); err == nil {
		execPath = installedPath
	}

	iconPath := filepath.Join(homeDir, ".local", "share", "icons", "clipboard-gnome.png")
	autostartContent := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Clipboard-Gnome
Comment=Hybrid Wayland Clipboard Manager
Exec=env WEBKIT_DISABLE_DMABUF_RENDERER=1 WEBKIT_DISABLE_COMPOSITING_MODE=1 %s --hidden
Icon=%s
Terminal=false
Categories=Utility;
X-GNOME-Autostart-enabled=true
`, execPath, iconPath)

	desktopPath := filepath.Join(autostartDir, "clipboard-gnome.desktop")
	err = os.WriteFile(desktopPath, []byte(autostartContent), 0644)
	if err != nil {
		logger.Warn("Failed to write autostart desktop file: %v", err)
	} else {
		logger.Info("Autostart desktop entry verified at %s", desktopPath)
	}

	appsContent := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Clipboard-Gnome
Comment=Hybrid Wayland Clipboard Manager
Exec=env WEBKIT_DISABLE_DMABUF_RENDERER=1 WEBKIT_DISABLE_COMPOSITING_MODE=1 %s
Icon=%s
Terminal=false
Categories=Utility;
`, execPath, iconPath)

	appsDir := filepath.Join(homeDir, ".local", "share", "applications")
	_ = os.MkdirAll(appsDir, 0755)
	appsDesktopPath := filepath.Join(appsDir, "clipboard-gnome.desktop")
	_ = os.WriteFile(appsDesktopPath, []byte(appsContent), 0644)
}

// --- Frontend Bindings ---

func (a *App) GetHistory(query, tag string, pinnedOnly bool, limit, offset int) []db.ClipboardItem {
	if a.database == nil {
		return []db.ClipboardItem{}
	}
	items, err := a.database.GetRecentItems(limit, offset, query, tag, pinnedOnly)
	if err != nil {
		logger.Error("GetHistory error: %v", err)
		return []db.ClipboardItem{}
	}
	logger.Info("GetHistory returned %d items (query=%q, tag=%q, pinnedOnly=%v)", len(items), query, tag, pinnedOnly)
	return items
}

func (a *App) TriggerPaste(id int64) error {
	if a.database == nil {
		return fmt.Errorf("database not initialized")
	}

	item, err := a.database.GetItemByID(id)
	if err != nil {
		return err
	}

	// 1. Immediately hide window to yield focus back to target app
	if a.ctx != nil {
		runtime.WindowHide(a.ctx)
	}

	// 2. Call DBus to inject paste
	if a.dbusClient != nil {
		err = a.dbusClient.InjectPaste(item.Type, item.Content)
		if err != nil {
			logger.Error("TriggerPaste DBus error: %v", err)
			return err
		}
	} else {
		logger.Warn("DBus client unavailable to inject paste")
	}

	return nil
}

func (a *App) TogglePin(id int64) (bool, error) {
	if a.database == nil {
		return false, fmt.Errorf("database not initialized")
	}
	pinned, err := a.database.TogglePin(id)
	if err != nil {
		return false, err
	}
	return pinned, nil
}

func (a *App) DeleteItem(id int64) error {
	if a.database == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.database.DeleteItem(id)
}

func (a *App) ClearHistory() error {
	if a.database == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.database.ClearHistory()
}

func (a *App) AddTag(id int64, tag string) error {
	if a.database == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.database.AddTag(id, tag)
}

func (a *App) RemoveTag(id int64, tag string) error {
	if a.database == nil {
		return fmt.Errorf("database not initialized")
	}
	return a.database.RemoveTag(id, tag)
}

func (a *App) ToggleIncognito() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.incognito = !a.incognito
	logger.Info("Incognito mode toggled to %v", a.incognito)
	return a.incognito
}

func (a *App) IsIncognito() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.incognito
}

func (a *App) HideWindow() {
	if a.ctx != nil {
		runtime.WindowHide(a.ctx)
	}
}

func (a *App) GetStats() map[string]interface{} {
	if a.database == nil {
		return map[string]interface{}{"status": "db_offline"}
	}
	stats, err := a.database.GetStats()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	stats["incognito"] = a.IsIncognito()
	return stats
}

func (a *App) GetSettings() *settings.AppSettings {
	return settings.LoadSettings()
}

func (a *App) SaveSettings(s *settings.AppSettings) error {
	return settings.SaveSettings(s)
}

