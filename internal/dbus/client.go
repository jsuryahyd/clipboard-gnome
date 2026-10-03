package dbus

import (
	"fmt"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
	"clipboard-gnome/internal/logger"
)

const (
	DBusService          = "org.gnome.Shell"
	DBusObjectGnome      = "/org/gnome/Shell/Extensions/ClipboardGnome"
	DBusInterfaceGnome   = "org.gnome.Shell.Extensions.ClipboardGnome"
	DBusObjectLegacy     = "/org/gnome/Shell/Extensions/ClipboardGo"
	DBusInterfaceLegacy  = "org.gnome.Shell.Extensions.ClipboardGo"
)

type Client struct {
	conn       *dbus.Conn
	signalChan chan *dbus.Signal
	stopChan   chan struct{}
	mu         sync.Mutex
	isClosed   bool

	OnClipboardChanged func(itemType, content string)
	OnShowUI           func()
}

// NewClient initializes a new DBus client on the session bus.
func NewClient(onClipboardChanged func(itemType, content string), onShowUI func()) (*Client, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to session dbus: %w", err)
	}

	c := &Client{
		conn:               conn,
		signalChan:         make(chan *dbus.Signal, 100),
		stopChan:           make(chan struct{}),
		OnClipboardChanged: onClipboardChanged,
		OnShowUI:           onShowUI,
	}

	// Register signal channel with DBus connection
	conn.Signal(c.signalChan)

	// Add match rules for extension signals (both Gnome and legacy Go interfaces)
	interfaces := []string{DBusInterfaceGnome, DBusInterfaceLegacy}
	for _, iface := range interfaces {
		matchRules := []string{
			fmt.Sprintf("type='signal',interface='%s',member='ClipboardChanged'", iface),
			fmt.Sprintf("type='signal',interface='%s',member='ShowUI'", iface),
		}
		for _, rule := range matchRules {
			call := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)
			if call.Err != nil {
				logger.Warn("DBus AddMatch rule warning (%s): %v", rule, call.Err)
			}
		}
	}

	go c.listenLoop()

	logger.Info("DBus client initialized and listening for GNOME extension signals")
	return c, nil
}

func (c *Client) listenLoop() {
	for {
		select {
		case <-c.stopChan:
			return
		case sig, ok := <-c.signalChan:
			if !ok {
				return
			}
			if sig == nil {
				continue
			}

			logger.Debug("Received DBus signal: %s", sig.Name)

			if strings.HasSuffix(sig.Name, ".ClipboardChanged") {
				if len(sig.Body) >= 2 {
					itemType, ok1 := sig.Body[0].(string)
					content, ok2 := sig.Body[1].(string)
					if ok1 && ok2 && c.OnClipboardChanged != nil {
						c.OnClipboardChanged(itemType, content)
					}
				}
			} else if strings.HasSuffix(sig.Name, ".ShowUI") {
				if c.OnShowUI != nil {
					c.OnShowUI()
				}
			}
		}
	}
}

// ActivateWindow requests GNOME Shell extension to focus/raise the application window.
func (c *Client) ActivateWindow() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isClosed || c.conn == nil {
		return fmt.Errorf("dbus client is closed")
	}

	// Try ClipboardGnome interface first
	obj := c.conn.Object(DBusService, dbus.ObjectPath(DBusObjectGnome))
	var success bool
	call := obj.Call(DBusInterfaceGnome+".ActivateWindow", 0).Store(&success)
	if call == nil {
		logger.Debug("DBus ActivateWindow succeeded via ClipboardGnome (result: %v)", success)
		return nil
	}

	// Fallback to ClipboardGo legacy interface
	objLegacy := c.conn.Object(DBusService, dbus.ObjectPath(DBusObjectLegacy))
	callLegacy := objLegacy.Call(DBusInterfaceLegacy+".ActivateWindow", 0).Store(&success)
	if callLegacy == nil {
		logger.Debug("DBus ActivateWindow succeeded via legacy ClipboardGo (result: %v)", success)
		return nil
	}

	logger.Debug("DBus ActivateWindow calls failed: %v", call.Error())
	return call
}

// InjectPaste calls the GNOME Extension DBus method to set content and simulate Ctrl+V
func (c *Client) InjectPaste(itemType, content string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isClosed || c.conn == nil {
		return fmt.Errorf("dbus client is closed")
	}

	// Try ClipboardGnome first
	obj := c.conn.Object(DBusService, dbus.ObjectPath(DBusObjectGnome))
	call := obj.Call(DBusInterfaceGnome+".InjectPaste", 0, itemType, content)
	if call.Err == nil {
		logger.Info("DBus InjectPaste successfully sent to ClipboardGnome extension (type: %s)", itemType)
		return nil
	}

	// Fallback to ClipboardGo legacy
	objLegacy := c.conn.Object(DBusService, dbus.ObjectPath(DBusObjectLegacy))
	callLegacy := objLegacy.Call(DBusInterfaceLegacy+".InjectPaste", 0, itemType, content)
	if callLegacy.Err == nil {
		logger.Info("DBus InjectPaste successfully sent to legacy ClipboardGo extension (type: %s)", itemType)
		return nil
	}

	logger.Error("DBus InjectPaste call failed: %v", call.Err)
	return call.Err
}

// Close cleans up DBus connections
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isClosed {
		return
	}
	c.isClosed = true
	close(c.stopChan)

	if c.conn != nil {
		c.conn.RemoveSignal(c.signalChan)
		_ = c.conn.Close()
	}
}
