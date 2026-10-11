package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Josedzzz/baky/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

// newTestModel returns a model backed by a config file in a temp directory
func newTestModel(t *testing.T) Model {
	t.Helper()

	old := config.Path()
	config.SetPath(filepath.Join(t.TempDir(), "config.json"))
	t.Cleanup(func() { config.SetPath(old) })

	m := NewModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(Model)
}

// press sends keys to the model; a name like "enter" or "esc" is a special
// key, anything else is typed as text
func press(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, key := range keys {
		var msg tea.KeyMsg
		switch key {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		}
		updated, _ := m.Update(msg)
		m = updated.(Model)
	}
	return m
}

func assertShows(t *testing.T, m Model, texts ...string) {
	t.Helper()
	view := m.View()
	for _, text := range texts {
		if !strings.Contains(view, text) {
			t.Errorf("expected the screen to show %q, got:\n%s", text, view)
		}
	}
}

func TestMenuNavigation(t *testing.T) {
	m := newTestModel(t)
	assertShows(t, m, "MENÚ PRINCIPAL", "Respaldo de archivos", "Respaldo de configuraciones", "Servicio", "Ayuda", "Salir")

	// Every entry of every menu opens a screen that renders something
	for state, current := range menus {
		for i, entry := range current.entries {
			if entry.target == quitEntry {
				continue
			}
			m.state, m.cursor = state, i
			opened := press(t, m, "enter")
			if opened.state != entry.target {
				t.Errorf("%s: expected state %d, got %d", entry.label, entry.target, opened.state)
			}
			if !strings.Contains(opened.View(), "BAKY") && !isMenu(opened.state) {
				t.Errorf("%s: the screen rendered nothing", entry.label)
			}
		}
	}

	// A submenu remembers its place and esc walks back up
	m = newTestModel(t)
	m = press(t, m, "down", "enter")
	assertShows(t, m, "RESPALDO DE CONFIGURACIONES", "Carpetas vigiladas", "Historial de cambios", "Repositorio")
	m = press(t, m, "down", "down", "enter")
	assertShows(t, m, "REPOSITORIO DE CONFIGURACIONES")
	m = press(t, m, "esc")
	if m.state != configMenuView || m.cursor != 2 {
		t.Errorf("expected to return to the third entry of the config menu, got state %d cursor %d", m.state, m.cursor)
	}
	m = press(t, m, "esc")
	if m.state != menuView || m.cursor != 1 {
		t.Errorf("expected to return to the second entry of the main menu, got state %d cursor %d", m.state, m.cursor)
	}

	m = press(t, m, "?")
	assertShows(t, m, "AYUDA", "Respaldo de configuraciones")
	m = press(t, m, "esc")

	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}); cmd == nil {
		t.Errorf("expected q to quit from the main menu")
	}
}

func TestAddAndRemoveWatchedFolder(t *testing.T) {
	m := newTestModel(t)
	source := filepath.Join(t.TempDir(), "openvpn")
	os.MkdirAll(source, 0o755)

	m.state = watchesView
	assertShows(t, m, "No hay carpetas vigiladas", "Aún no hay un repositorio configurado")

	// A path that does not exist is rejected in the first step
	m = press(t, m, "a", "/no/existe", "enter")
	assertShows(t, m, "Paso 1 de 2", "no existe o no es una carpeta")

	m = press(t, m, "esc", "a", source, "enter")
	assertShows(t, m, "Paso 2 de 2", source)
	if m.watchInput.Value() != "openvpn" {
		t.Errorf("expected the folder name as the suggested destination, got %q", m.watchInput.Value())
	}

	m = press(t, m, "enter")
	assertShows(t, m, "CARPETAS VIGILADAS", source+"  →  openvpn", "agregada")

	cb, _ := config.GetConfigBackup()
	if len(cb.Watches) != 1 || cb.Watches[0].Source != source || cb.Watches[0].Dest != "openvpn" {
		t.Fatalf("watch not saved: %+v", cb.Watches)
	}

	// Removing asks first, and "n" cancels
	m = press(t, m, "d")
	assertShows(t, m, "¿Dejar de vigilar la carpeta", "No se borra ningún archivo")
	m = press(t, m, "n")
	if cb, _ := config.GetConfigBackup(); len(cb.Watches) != 1 {
		t.Errorf("cancelling must keep the watch")
	}

	m = press(t, m, "d", "s")
	assertShows(t, m, "ya no se vigila", "No hay carpetas vigiladas")
	if cb, _ := config.GetConfigBackup(); len(cb.Watches) != 0 {
		t.Errorf("expected the watch to be removed, got %+v", cb.Watches)
	}
}

func TestManagePaths(t *testing.T) {
	m := newTestModel(t)
	m.state = managePathsView

	m = press(t, m, "a", "/home/user/docs", "enter")
	assertShows(t, m, "/home/user/docs", "[diario]", "Ruta agregada")

	m = press(t, m, "f")
	assertShows(t, m, "[semanal]")
	m = press(t, m, "f")
	assertShows(t, m, "[al cambiar]")

	paths, _ := config.GetPaths()
	if len(paths) != 1 || paths[0].Frequency != config.FreqOnChange {
		t.Fatalf("path not saved: %+v", paths)
	}

	m = press(t, m, "d")
	assertShows(t, m, "Aún no hay rutas agregadas", "Ruta eliminada")
}

func TestConfigRepoScreen(t *testing.T) {
	m := newTestModel(t)
	m.state = configRepoView
	assertShows(t, m, "sin configurar")

	notRepo := t.TempDir()
	m = press(t, m, "e", notRepo, "enter")
	assertShows(t, m, notRepo, "no es un repositorio Git")

	if cb, _ := config.GetConfigBackup(); cb.RepoDir != notRepo {
		t.Errorf("repository path not saved: %+v", cb)
	}
}

func TestLogsScreen(t *testing.T) {
	m := newTestModel(t)

	m.state = logsView
	updated, _ := m.Update(logsLoadedMsg{err: os.ErrNotExist})
	assertShows(t, updated.(Model), "No se pudo leer el log")

	updated, _ = m.Update(logsLoadedMsg{lines: []string{
		"2026-07-24 21:46:14 [INFO] Commit realizado: actualizado nat.conf -> firewall/nat.conf.",
		"2026-07-24 21:46:17 [ERROR] Fallo al hacer push.",
	}})
	assertShows(t, updated.(Model), "Commit realizado", "Fallo al hacer push")
}
