package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

type ClipboardItem struct {
	ID        int64    `json:"id"`
	Type      string   `json:"type"`      // "text" or "image"
	Content   string   `json:"content"`   // text string or image filepath
	SourceApp string   `json:"source_app"`
	CreatedAt string   `json:"created_at"`
	Pinned    bool     `json:"pinned"`
	Tags      []string `json:"tags"`
}

type DB struct {
	conn *sql.DB
}

// InitDB initializes the SQLite database connection and sets up tables and indices.
func InitDB(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	conn, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Enable WAL mode and foreign keys for high performance and concurrency
	if _, err := conn.Exec("PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;"); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to set pragmas: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS clipboard_items (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		type TEXT NOT NULL CHECK(type IN ('text', 'image')),
		content TEXT NOT NULL,
		source_app TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		pinned BOOLEAN DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS tags (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		item_id INTEGER NOT NULL,
		tag TEXT NOT NULL,
		FOREIGN KEY(item_id) REFERENCES clipboard_items(id) ON DELETE CASCADE,
		UNIQUE(item_id, tag)
	);

	CREATE INDEX IF NOT EXISTS idx_created_at ON clipboard_items(created_at DESC);
	CREATE INDEX IF NOT EXISTS idx_pinned ON clipboard_items(pinned);
	`

	if _, err := conn.Exec(schema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to execute schema migration: %w", err)
	}

	return &DB{conn: conn}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	if d.conn != nil {
		return d.conn.Close()
	}
	return nil
}

// InsertItem inserts a new clipboard item, preventing immediate duplicate entries.
func (d *DB) InsertItem(itemType, content, sourceApp string) (*ClipboardItem, error) {
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("content cannot be empty")
	}

	// Check most recent item to prevent duplicate sequential saves
	var lastContent string
	var lastID int64
	err := d.conn.QueryRow("SELECT id, content FROM clipboard_items ORDER BY id DESC LIMIT 1").Scan(&lastID, &lastContent)
	if err == nil && lastContent == content {
		// Update created_at timestamp instead of inserting duplicate
		_, _ = d.conn.Exec("UPDATE clipboard_items SET created_at = CURRENT_TIMESTAMP WHERE id = ?", lastID)
		return d.GetItemByID(lastID)
	}

	res, err := d.conn.Exec(
		"INSERT INTO clipboard_items (type, content, source_app, created_at, pinned) VALUES (?, ?, ?, CURRENT_TIMESTAMP, 0)",
		itemType, content, sourceApp,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert clipboard item: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}

	return d.GetItemByID(id)
}

// GetItemByID retrieves a single item by ID including its tags.
func (d *DB) GetItemByID(id int64) (*ClipboardItem, error) {
	row := d.conn.QueryRow(
		"SELECT id, type, content, COALESCE(source_app, ''), created_at, pinned FROM clipboard_items WHERE id = ?",
		id,
	)

	var item ClipboardItem
	var pinnedInt int
	if err := row.Scan(&item.ID, &item.Type, &item.Content, &item.SourceApp, &item.CreatedAt, &pinnedInt); err != nil {
		return nil, fmt.Errorf("item not found: %w", err)
	}
	item.Pinned = (pinnedInt == 1)
	item.Tags = d.getTagsForItem(item.ID)

	return &item, nil
}

// GetRecentItems fetches filtered and paginated clipboard history items.
func (d *DB) GetRecentItems(limit, offset int, query, tag string, pinnedOnly bool) ([]ClipboardItem, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var conditions []string
	var args []interface{}

	if query != "" {
		conditions = append(conditions, "content LIKE ?")
		args = append(args, "%"+query+"%")
	}

	if pinnedOnly {
		conditions = append(conditions, "pinned = 1")
	}

	if tag != "" {
		conditions = append(conditions, "id IN (SELECT item_id FROM tags WHERE tag = ?)")
		args = append(args, tag)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Always prioritize pinned items first, then by creation time descending
	queryStr := fmt.Sprintf(`
		SELECT id, type, content, COALESCE(source_app, ''), created_at, pinned 
		FROM clipboard_items 
		%s 
		ORDER BY pinned DESC, created_at DESC 
		LIMIT ? OFFSET ?`, whereClause)

	args = append(args, limit, offset)

	rows, err := d.conn.Query(queryStr, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query clipboard items: %w", err)
	}
	defer rows.Close()

	var items []ClipboardItem
	for rows.Next() {
		var item ClipboardItem
		var pinnedInt int
		if err := rows.Scan(&item.ID, &item.Type, &item.Content, &item.SourceApp, &item.CreatedAt, &pinnedInt); err != nil {
			return nil, err
		}
		item.Pinned = (pinnedInt == 1)
		item.Tags = d.getTagsForItem(item.ID)
		items = append(items, item)
	}

	if items == nil {
		items = []ClipboardItem{}
	}

	return items, nil
}

// TogglePin toggles the pinned status of a clipboard item.
func (d *DB) TogglePin(id int64) (bool, error) {
	item, err := d.GetItemByID(id)
	if err != nil {
		return false, err
	}

	newPinned := !item.Pinned
	if newPinned {
		_, err = d.conn.Exec("UPDATE clipboard_items SET pinned = 1 WHERE id = ?", id)
	} else {
		// When unpinning: refresh created_at to CURRENT_TIMESTAMP so the item lands
		// at the top of the unpinned stack instead of disappearing into old history
		_, err = d.conn.Exec("UPDATE clipboard_items SET pinned = 0, created_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	}
	if err != nil {
		return false, fmt.Errorf("failed to update pinned status: %w", err)
	}

	return newPinned, nil
}

// DeleteItem deletes a clipboard item by ID.
func (d *DB) DeleteItem(id int64) error {
	_, err := d.conn.Exec("DELETE FROM clipboard_items WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete item: %w", err)
	}
	return nil
}

// ClearHistory deletes all unpinned items from database.
func (d *DB) ClearHistory() error {
	_, err := d.conn.Exec("DELETE FROM clipboard_items WHERE pinned = 0")
	if err != nil {
		return fmt.Errorf("failed to clear unpinned history: %w", err)
	}
	return nil
}

// AddTag adds a tag to an item.
func (d *DB) AddTag(itemID int64, tag string) error {
	tag = strings.TrimSpace(strings.ToLower(tag))
	if tag == "" {
		return fmt.Errorf("tag cannot be empty")
	}
	_, err := d.conn.Exec("INSERT OR IGNORE INTO tags (item_id, tag) VALUES (?, ?)", itemID, tag)
	return err
}

// RemoveTag removes a tag from an item.
func (d *DB) RemoveTag(itemID int64, tag string) error {
	_, err := d.conn.Exec("DELETE FROM tags WHERE item_id = ? AND tag = ?", itemID, strings.ToLower(tag))
	return err
}

// getTagsForItem retrieves all tags for an item.
func (d *DB) getTagsForItem(itemID int64) []string {
	rows, err := d.conn.Query("SELECT tag FROM tags WHERE item_id = ? ORDER BY tag ASC", itemID)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err == nil {
			tags = append(tags, tag)
		}
	}
	if tags == nil {
		tags = []string{}
	}
	return tags
}

// Cleanup purges unpinned items older than retentionDays, limits DB size, and vacuums.
func (d *DB) Cleanup(retentionDays int, maxItemSizeBytes int64) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	if maxItemSizeBytes <= 0 {
		maxItemSizeBytes = 10 * 1024 * 1024 // 10 MB
	}

	// Delete unpinned items older than retentionDays
	res, err := d.conn.Exec(
		"DELETE FROM clipboard_items WHERE pinned = 0 AND created_at < datetime('now', '-' || ? || ' days')",
		retentionDays,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to purge old items: %w", err)
	}

	deletedCount, _ := res.RowsAffected()

	// Delete unpinned items with payload size > maxItemSizeBytes
	_, _ = d.conn.Exec(
		"DELETE FROM clipboard_items WHERE pinned = 0 AND length(content) > ?",
		maxItemSizeBytes,
	)

	// Run VACUUM to reclaim disk space
	_, _ = d.conn.Exec("VACUUM")

	return deletedCount, nil
}

// GetStats returns summary statistics about the DB.
func (d *DB) GetStats() (map[string]interface{}, error) {
	var total, pinned int64
	_ = d.conn.QueryRow("SELECT COUNT(*) FROM clipboard_items").Scan(&total)
	_ = d.conn.QueryRow("SELECT COUNT(*) FROM clipboard_items WHERE pinned = 1").Scan(&pinned)

	return map[string]interface{}{
		"total_items":  total,
		"pinned_items": pinned,
	}, nil
}
