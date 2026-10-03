#!/bin/bash
set -e

# Support PATH for Go and Linuxbrew if available
export PATH=/home/linuxbrew/.linuxbrew/bin:/usr/local/go/bin:$HOME/go/bin:$PATH

# Ensure Wayland/X11/DBus environment if running from non-login or subagent shell
if [ -z "$XDG_RUNTIME_DIR" ]; then
    export XDG_RUNTIME_DIR="/run/user/$(id -u)"
fi
if [ -z "$WAYLAND_DISPLAY" ] && [ -S "$XDG_RUNTIME_DIR/wayland-0" ]; then
    export WAYLAND_DISPLAY="wayland-0"
fi
if [ -z "$DISPLAY" ]; then
    export DISPLAY=":0"
fi
if [ -z "$DBUS_SESSION_BUS_ADDRESS" ] && [ -S "$XDG_RUNTIME_DIR/bus" ]; then
    export DBUS_SESSION_BUS_ADDRESS="unix:path=$XDG_RUNTIME_DIR/bus"
fi

FORCE=0
for arg in "$@"; do
    case "$arg" in
        -y|--yes|--force)
            FORCE=1
            ;;
        -h|--help)
            echo "Usage: ./install.sh [OPTIONS]"
            echo "  -y, --yes    Replace existing installation without interactive prompt"
            echo "  -h, --help   Show this help message"
            exit 0
            ;;
    esac
done

echo "Clipboard-Gnome Installation Script"
echo "-----------------------------------"

# 1. Check dependencies
deps=(go npm sqlite3 zip unzip curl)
missing=0
for dep in "${deps[@]}"; do
    if ! command -v "$dep" &> /dev/null; then
        echo "$dep could not be found. Please install it."
        missing=1
    fi
done

if [ $missing -eq 1 ]; then
    echo "Please install the missing dependencies and run the script again."
    exit 1
fi

echo "Dependencies satisfied."

# Clean corrupted fontconfig cache symlinks if present (prevents WebKitWebProcess 100% CPU hang on Zorin/Ubuntu)
if [ -d "$HOME/.cache/fontconfig" ]; then
    if find "$HOME/.cache/fontconfig" -maxdepth 1 -type l 2>/dev/null | grep -q .; then
        echo "Detected corrupted fontconfig cache symlinks; repairing cache..."
        rm -rf "$HOME/.cache/fontconfig"
        if command -v fc-cache >/dev/null 2>&1; then
            fc-cache -vr >/dev/null 2>&1 || true
        fi
    fi
fi

# 2. Version Detection
TARGET_BIN="$HOME/.local/bin/clipboard-gnome"
NEW_VERSION="1.0.1"
if [ -f "wails.json" ]; then
    DETECTED_VER=$(grep -o '"productVersion": *"[^"]*"' wails.json | head -1 | cut -d'"' -f4)
    if [ -n "$DETECTED_VER" ]; then
        NEW_VERSION="$DETECTED_VER"
    fi
fi

EXISTING_VERSION=""
IS_INSTALLED=0

if [ -f "$TARGET_BIN" ]; then
    IS_INSTALLED=1
    if EXISTING_OUT=$("$TARGET_BIN" --version 2>/dev/null); then
        EXISTING_VERSION=$(echo "$EXISTING_OUT" | awk '{print $NF}')
    fi
    if [ -z "$EXISTING_VERSION" ]; then
        EXISTING_VERSION="1.0.0 (legacy/unversioned)"
    fi
fi

RUNNING_PIDS=$(pgrep -u "$USER" -x clipboard-gnome || true)

# 3. Check for replacement confirmation
if [ $IS_INSTALLED -eq 1 ] || [ -n "$RUNNING_PIDS" ]; then
    echo ""
    echo "Existing installation found:"
    if [ $IS_INSTALLED -eq 1 ]; then
        echo "  - Installed version: $EXISTING_VERSION ($TARGET_BIN)"
    fi
    if [ -n "$RUNNING_PIDS" ]; then
        echo "  - Running process PID(s): $RUNNING_PIDS"
    fi
    echo "  - New version:       $NEW_VERSION"
    echo ""

    if [ $FORCE -ne 1 ]; then
        if [ -t 0 ]; then
            read -r -p "Do you want to replace the existing installation with version $NEW_VERSION? [Y/n] " answer
            answer=${answer:-Y}
            if [[ ! "$answer" =~ ^[Yy]$ ]]; then
                echo "Installation cancelled by user."
                exit 0
            fi
        else
            echo "Non-interactive environment detected; proceeding with replacement of version $EXISTING_VERSION by $NEW_VERSION."
        fi
    fi
    echo "Proceeding with replacement..."
fi

# 4. Stop existing running instances
if [ -n "$RUNNING_PIDS" ]; then
    echo "Stopping existing clipboard-gnome process(es) ($RUNNING_PIDS)..."
    kill $RUNNING_PIDS 2>/dev/null || true
    sleep 1
    kill -9 $RUNNING_PIDS 2>/dev/null || true
fi

LEGACY_PIDS=$(pgrep -u "$USER" -x clipboard-go || true)
if [ -n "$LEGACY_PIDS" ]; then
    echo "Stopping legacy clipboard-go process(es) ($LEGACY_PIDS)..."
    kill $LEGACY_PIDS 2>/dev/null || true
fi

# 5. Disable legacy extension if enabled
if command -v gnome-extensions &>/dev/null; then
    if gnome-extensions list 2>/dev/null | grep -q "clipboard-go@surya.dev"; then
        echo "Disabling legacy clipboard-go GNOME extension..."
        gnome-extensions disable clipboard-go@surya.dev 2>/dev/null || true
    fi
fi

# 6. Build
echo "Building Clipboard-Gnome (v$NEW_VERSION)..."
make build

# 7. Install Extension
echo "Installing GNOME Extension..."
make install

# 8. Enable GNOME Extension
if command -v gnome-extensions &>/dev/null; then
    echo "Enabling GNOME Extension (clipboard-gnome@surya.dev)..."
    gnome-extensions enable clipboard-gnome@surya.dev 2>/dev/null || true
fi

# 9. Copy icon
echo "Installing icon..."
mkdir -p ~/.local/share/icons
cp assets/appicon.png ~/.local/share/icons/clipboard-gnome.png

# 10. Copy binary to path
echo "Installing binary to ~/.local/bin..."
mkdir -p ~/.local/bin
cp build/bin/clipboard-gnome "$TARGET_BIN"

# 11. Install desktop entries (applications launcher and autostart)
echo "Installing desktop launcher and autostart entries..."
mkdir -p ~/.local/share/applications ~/.config/autostart
cat <<EOF > ~/.local/share/applications/clipboard-gnome.desktop
[Desktop Entry]
Type=Application
Name=Clipboard-Gnome
Comment=Hybrid Wayland Clipboard Manager
Exec=env WEBKIT_DISABLE_DMABUF_RENDERER=1 WEBKIT_DISABLE_COMPOSITING_MODE=1 $TARGET_BIN
Icon=$HOME/.local/share/icons/clipboard-gnome.png
Terminal=false
Categories=Utility;
EOF

cat <<EOF > ~/.config/autostart/clipboard-gnome.desktop
[Desktop Entry]
Type=Application
Name=Clipboard-Gnome
Comment=Hybrid Wayland Clipboard Manager
Exec=env WEBKIT_DISABLE_DMABUF_RENDERER=1 WEBKIT_DISABLE_COMPOSITING_MODE=1 $TARGET_BIN --hidden
Icon=$HOME/.local/share/icons/clipboard-gnome.png
Terminal=false
Categories=Utility;
X-GNOME-Autostart-enabled=true
EOF

# 12. Start application
echo "Starting Clipboard-Gnome (v$NEW_VERSION)..."
(nohup "$TARGET_BIN" --hidden > /dev/null 2>&1 &)

echo ""
echo "-----------------------------------"
echo "Installation Complete! (Version $NEW_VERSION installed)"
echo "If this is your first time installing, please restart GNOME Shell (Alt+F2, r, Enter on X11, or log out/in on Wayland)."
