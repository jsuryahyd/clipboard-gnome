//go:build linux

package compat

/*
#cgo pkg-config: gtk+-3.0

#include <gtk/gtk.h>

static void set_window_skip_taskbar() {
	GList *windows = gtk_window_list_toplevels();
	for (GList *l = windows; l != NULL; l = l->next) {
		if (GTK_IS_WINDOW(l->data)) {
			GtkWindow *win = GTK_WINDOW(l->data);
			gtk_window_set_skip_taskbar_hint(win, TRUE);
			gtk_window_set_skip_pager_hint(win, TRUE);
			gtk_window_set_keep_above(win, TRUE);
			gtk_window_set_type_hint(win, GDK_WINDOW_TYPE_HINT_UTILITY);
		}
	}
	if (windows) {
		g_list_free(windows);
	}
}
*/
import "C"
import (
	"sync"
	"time"
)

var (
	hintsOnce sync.Once
)

// ApplyGtkTaskbarHints ensures all GTK windows for the application are hidden from
// the taskbar, dock, and workspace pager, functioning strictly as a utility window.
func ApplyGtkTaskbarHints() {
	go func() {
		// Periodically apply GTK hints during startup to catch the window when mapped by Wails
		for i := 0; i < 15; i++ {
			time.Sleep(200 * time.Millisecond)
			C.set_window_skip_taskbar()
		}
	}()
}
