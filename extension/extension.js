import { Extension } from 'resource:///org/gnome/shell/extensions/extension.js';
import St from 'gi://St';
import Clutter from 'gi://Clutter';
import GLib from 'gi://GLib';
import Gio from 'gi://Gio';
import Meta from 'gi://Meta';
import Shell from 'gi://Shell';
import * as Main from 'resource:///org/gnome/shell/ui/main.js';

const DBUS_IFACE_GNOME = `
<node>
    <interface name="org.gnome.Shell.Extensions.ClipboardGnome">
        <signal name="ClipboardChanged">
            <arg type="s" name="type"/>
            <arg type="s" name="content"/>
        </signal>
        <signal name="ShowUI"/>
        <method name="InjectPaste">
            <arg type="s" direction="in" name="type"/>
            <arg type="s" direction="in" name="content"/>
        </method>
        <method name="ActivateWindow">
            <arg type="b" direction="out" name="success"/>
        </method>
    </interface>
</node>`;

const DBUS_IFACE_LEGACY = `
<node>
    <interface name="org.gnome.Shell.Extensions.ClipboardGo">
        <signal name="ClipboardChanged">
            <arg type="s" name="type"/>
            <arg type="s" name="content"/>
        </signal>
        <signal name="ShowUI"/>
        <method name="InjectPaste">
            <arg type="s" direction="in" name="type"/>
            <arg type="s" direction="in" name="content"/>
        </method>
        <method name="ActivateWindow">
            <arg type="b" direction="out" name="success"/>
        </method>
    </interface>
</node>`;

export default class ClipboardExtension extends Extension {
    enable() {
        console.log('[Clipboard-Gnome] Enabling extension bridge for GNOME 45+...');
        this._clipboard = St.Clipboard.get_default();
        this._lastContent = "";
        this._pollTimerId = 0;
        this._dbusImpl = null;
        this._dbusImplLegacy = null;
        this._keybindingName = 'toggle-clipboard-gnome';

        // 1. Export DBus Interfaces on Session Bus
        try {
            this._dbusImpl = Gio.DBusExportedObject.wrapJSObject(DBUS_IFACE_GNOME, this);
            this._dbusImpl.export(Gio.DBus.session, '/org/gnome/Shell/Extensions/ClipboardGnome');
            console.log('[Clipboard-Gnome] DBus interface exported at /org/gnome/Shell/Extensions/ClipboardGnome');
        } catch (e) {
            console.error(`[Clipboard-Gnome] Error exporting ClipboardGnome DBus interface: ${e}`);
        }

        try {
            this._dbusImplLegacy = Gio.DBusExportedObject.wrapJSObject(DBUS_IFACE_LEGACY, this);
            this._dbusImplLegacy.export(Gio.DBus.session, '/org/gnome/Shell/Extensions/ClipboardGo');
            console.log('[Clipboard-Gnome] DBus interface exported at /org/gnome/Shell/Extensions/ClipboardGo');
        } catch (e) {
            console.warn(`[Clipboard-Gnome] Legacy DBus interface export skipped: ${e}`);
        }

        // 2. Start Clipboard Polling
        this._startClipboardMonitoring();

        // 3. Register Global Hotkey
        this._registerHotkey();
    }

    disable() {
        console.log('[Clipboard-Gnome] Disabling extension bridge...');

        // 1. Stop Clipboard Polling
        this._stopClipboardMonitoring();

        // 2. Unregister Hotkey
        this._unregisterHotkey();

        // 3. Unexport DBus
        if (this._dbusImpl) {
            this._dbusImpl.unexport();
            this._dbusImpl = null;
        }
        if (this._dbusImplLegacy) {
            this._dbusImplLegacy.unexport();
            this._dbusImplLegacy = null;
        }
    }

    _startClipboardMonitoring() {
        this._pollTimerId = GLib.timeout_add(GLib.PRIORITY_DEFAULT, 500, () => {
            this._checkClipboardContent();
            return GLib.SOURCE_CONTINUE;
        });
    }

    _stopClipboardMonitoring() {
        if (this._pollTimerId > 0) {
            GLib.source_remove(this._pollTimerId);
            this._pollTimerId = 0;
        }
    }

    _emitSignal(name, variant) {
        if (this._dbusImpl) {
            try {
                this._dbusImpl.emit_signal(name, variant);
            } catch (e) {
                // Ignore
            }
        }
        if (this._dbusImplLegacy) {
            try {
                this._dbusImplLegacy.emit_signal(name, variant);
            } catch (e) {
                // Ignore
            }
        }
    }

    _checkClipboardContent() {
        try {
            this._clipboard.get_text(St.ClipboardType.CLIPBOARD, (clipboard, text) => {
                if (text && text.trim().length > 0 && text !== this._lastContent) {
                    this._lastContent = text;
                    this._emitSignal('ClipboardChanged', new GLib.Variant('(ss)', ['text', text]));
                }
            });
        } catch (e) {
            // Ignore temporary read errors
        }
    }

    _registerHotkey() {
        try {
            const settings = this.getSettings('org.gnome.shell.extensions.clipboard-gnome');
            Main.wm.addKeybinding(
                this._keybindingName,
                settings,
                Meta.KeyBindingFlags.NONE,
                Shell.ActionMode.ALL,
                () => {
                    console.log('[Clipboard-Gnome] Global hotkey pressed, emitting ShowUI signal');
                    this._emitSignal('ShowUI', null);
                    this._activateClipboardWindow();
                    GLib.timeout_add(GLib.PRIORITY_DEFAULT, 100, () => {
                        this._activateClipboardWindow();
                        return GLib.SOURCE_REMOVE;
                    });
                }
            );
        } catch (e) {
            console.warn(`[Clipboard-Gnome] Keybinding warning: ${e}`);
        }
    }

    _unregisterHotkey() {
        try {
            Main.wm.removeKeybinding(this._keybindingName);
        } catch (e) {
            // Ignore cleanup errors
        }
    }

    ActivateWindow() {
        return this._activateClipboardWindow();
    }

    _activateClipboardWindow() {
        try {
            const appSys = Shell.AppSystem.get_default();
            const app = appSys.lookup_app('clipboard-gnome.desktop');
            if (app) {
                const windows = app.get_windows();
                if (windows && windows.length > 0) {
                    Main.activateWindow(windows[0]);
                    return true;
                }
            }

            const actors = global.get_window_actors();
            for (const actor of actors) {
                const win = actor.meta_window;
                if (!win) continue;
                const wmClass = (win.get_wm_class() || '').toLowerCase();
                const title = (win.get_title() || '').toLowerCase();
                if (wmClass.includes('clipboard-gnome') || title.includes('clipboard-gnome')) {
                    Main.activateWindow(win);
                    return true;
                }
            }
        } catch (e) {
            console.warn(`[Clipboard-Gnome] Activate window error: ${e}`);
        }
        return false;
    }

    // DBus Method: InjectPaste
    InjectPaste(type, content) {
        console.log(`[Clipboard-Gnome] InjectPaste invoked (type=${type}, len=${content.length})`);
        
        this._lastContent = content;
        this._clipboard.set_text(St.ClipboardType.CLIPBOARD, content);

        console.warn('[Clipboard-Gnome] Auto-paste via Clutter synthetic event disabled to prevent GNOME Shell Wayland crash.');
    }
}
