#!/bin/bash
echo "Clipboard-Gnome Uninstallation Script"
echo "----------------------------------"

STATE_DIR="$HOME/.local/state/clipboard-gnome"
DB_PATH="$STATE_DIR/clipboard.db"
DESKTOP_DIR="$HOME/Desktop"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Stop running processes
echo "Stopping running Clipboard-Gnome processes..."
pkill -u "$USER" -x clipboard-gnome 2>/dev/null || true
pkill -u "$USER" -x clipboard-go 2>/dev/null || true

# Disable extensions
if command -v gnome-extensions &>/dev/null; then
    echo "Disabling GNOME extensions..."
    gnome-extensions disable clipboard-gnome@surya.dev 2>/dev/null || true
    gnome-extensions disable clipboard-go@surya.dev 2>/dev/null || true
fi

# Copy DB to CSV
if [ -f "$DB_PATH" ]; then
    echo "Exporting SQLite database to CSV..."
    CSV_FILE="$DESKTOP_DIR/clipboard_history_$TIMESTAMP.csv"
    if command -v sqlite3 &> /dev/null; then
        sqlite3 -header -csv "$DB_PATH" "SELECT * FROM clipboard_items;" > "$CSV_FILE"
        echo "Database exported to $CSV_FILE"
    else
        echo "sqlite3 not found. Skipping database export."
    fi
fi

# Zip image files (if any images are stored in state dir)
echo "Archiving image files (if any)..."
IMAGE_ZIP="$DESKTOP_DIR/clipboard_images_$TIMESTAMP.zip"
if [ -d "$STATE_DIR" ]; then
    find "$STATE_DIR" -type f \( -iname \*.jpg -o -iname \*.png -o -iname \*.jpeg \) -print0 | xargs -0r zip "$IMAGE_ZIP" 2>/dev/null || true
    if [ -f "$IMAGE_ZIP" ]; then
        echo "Images archived to $IMAGE_ZIP"
    else
        echo "No images found to archive."
    fi
fi

# Remove application and extension
echo "Removing application files..."
rm -f "$HOME/.local/bin/clipboard-gnome"
rm -f "$HOME/.config/autostart/clipboard-gnome.desktop"
rm -f "$HOME/.local/share/applications/clipboard-gnome.desktop"
rm -f "$HOME/.local/state/clipboard-gnome/clipboard-gnome.sock"
rm -rf "$HOME/.local/share/gnome-shell/extensions/clipboard-gnome@surya.dev"

echo "Uninstallation Complete!"
echo "Note: Your state directory at $STATE_DIR has been preserved."
