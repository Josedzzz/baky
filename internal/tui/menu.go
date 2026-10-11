package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// menuEntry is one option of a menu and the screen it opens
type menuEntry struct {
	label  string
	target sessionState
}

// quitEntry is the target of the menu option that closes the program
const quitEntry sessionState = -1

// menu is a list of options with a title
type menu struct {
	title   string
	entries []menuEntry
}

// menus holds the main menu and its submenus
var menus = map[sessionState]menu{
	menuView: {
		title: " MENÚ PRINCIPAL ",
		entries: []menuEntry{
			{"Respaldo de archivos", filesMenuView},
			{"Respaldo de configuraciones", configMenuView},
			{"Servicio", serviceMenuView},
			{"Ayuda", helpView},
			{"Salir", quitEntry},
		},
	},
	filesMenuView: {
		title: " RESPALDO DE ARCHIVOS ",
		entries: []menuEntry{
			{"Gestionar rutas", managePathsView},
			{"Respaldar ahora", backupFilesView},
			{"Ver respaldos / Restaurar", viewBackupsView},
			{"Destino de respaldo", configureBackupDestView},
		},
	},
	configMenuView: {
		title: " RESPALDO DE CONFIGURACIONES ",
		entries: []menuEntry{
			{"Carpetas vigiladas", watchesView},
			{"Historial de cambios", configHistoryView},
			{"Repositorio", configRepoView},
		},
	},
	serviceMenuView: {
		title: " SERVICIO ",
		entries: []menuEntry{
			{"Estado del servicio", serviceStatusView},
			{"Logs en vivo", logsView},
		},
	},
}

// isMenu reports whether a state is a menu rather than a screen
func isMenu(state sessionState) bool {
	_, ok := menus[state]
	return ok
}

// handleMenuUpdate processes key messages in the main menu and the submenus
func (m Model) handleMenuUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	entries := menus[m.state].entries

	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(entries)-1 {
			m.cursor++
		}
	case "enter":
		target := entries[m.cursor].target
		if target == quitEntry {
			m.quitting = true
			return m, tea.Quit
		}
		m.menuCursors[m.state] = m.cursor
		return m.open(target)
	case "?":
		m.menuCursors[m.state] = m.cursor
		return m.open(helpView)
	case "esc":
		if m.state != menuView {
			return m.back(menuView)
		}
	case "q":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// back returns to a menu, restoring the option that was selected in it
func (m Model) back(menuState sessionState) (tea.Model, tea.Cmd) {
	m.state = menuState
	m.cursor = m.menuCursors[menuState]
	m.message = ""
	return m, nil
}

// open switches to a menu or screen and loads what it needs to show
func (m Model) open(target sessionState) (tea.Model, tea.Cmd) {
	m.state = target
	m.message = ""

	switch target {
	case filesMenuView, configMenuView, serviceMenuView:
		m.cursor = m.menuCursors[target]
	case managePathsView:
		m.reloadPaths()
		m.pathsCursor = 0
	case backupFilesView:
		m.reloadPaths()
		m.pathsCursor = 0
		m.historyOffset = 0
	case viewBackupsView:
		m.loadBackups()
		m.backupsCursor = 0
		m.backupsOffset = 0
	case configureBackupDestView:
		m.reloadPaths()
	case watchesView:
		m.reloadConfigBackup()
		m.watchesCursor = 0
	case configRepoView:
		m.reloadConfigBackup()
	case configHistoryView:
		m.reloadConfigBackup()
		return m, loadCommitsCmd(m.configBackup.RepoDir)
	case serviceStatusView:
		m.serviceLoading = true
		return m, queryServiceCmd()
	case logsView:
		return m, tea.Batch(loadLogsCmd(), tickLogsCmd())
	}
	return m, nil
}

// renderMenu renders the main menu or a submenu
func (m Model) renderMenu(body *strings.Builder) {
	current := menus[m.state]

	if m.state == menuView {
		body.WriteString(asciiStyle.Render(logo) + "\n")
	} else {
		body.WriteString(miniLogoStyle.Render(miniLogo) + "\n")
	}
	body.WriteString(titleStyle.Render(current.title) + "\n\n")

	for i, entry := range current.entries {
		if m.cursor == i {
			fmt.Fprintf(body, "%s\n", selectedItemStyle.Render(entry.label))
		} else {
			fmt.Fprintf(body, "%s\n", itemStyle.Render(entry.label))
		}
	}

	if m.state == menuView {
		body.WriteString(statusStyle.Render(m.summary()) + "\n")
	}

	if m.message != "" {
		style := errorStyle
		if m.isSuccess {
			style = successStyle
		}
		body.WriteString(style.Render(m.message) + "\n")
	}

	if m.state == menuView {
		body.WriteString(footerStyle.Render("\n↑/↓: navegar • enter: seleccionar • ?: ayuda • q: salir"))
	} else {
		body.WriteString(footerStyle.Render("\n↑/↓: navegar • enter: seleccionar • esc: volver • q: salir"))
	}
}

// summary describes the current setup in the main menu
func (m Model) summary() string {
	dest := "sin configurar"
	if m.backupDest != "" {
		dest = m.backupDest
	}

	serviceState := "consultando..."
	if m.serviceKnown {
		serviceState = m.service.Status
		if m.service.Active {
			serviceState = "activo"
		} else if m.service.Status == "inactive" {
			serviceState = "inactivo"
		}
	}

	return fmt.Sprintf("\nDestino de respaldo: %s\nRutas de archivos: %d • Carpetas vigiladas: %d\nServicio: %s",
		dest, len(m.paths), len(m.configBackup.Watches), serviceState)
}
