package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// helpSections lists the keyboard shortcuts of every screen
var helpSections = []struct {
	title string
	keys  [][2]string
}{
	{"General", [][2]string{
		{"↑/↓ o k/j", "navegar"},
		{"enter", "seleccionar o confirmar"},
		{"esc", "volver a la pantalla anterior"},
		{"q", "salir (en los menús)"},
		{"ctrl+c", "salir desde cualquier pantalla"},
	}},
	{"Respaldo de archivos", [][2]string{
		{"a / e / d", "agregar, editar o eliminar una ruta"},
		{"f", "cambiar la frecuencia: diario, semanal, al cambiar"},
		{"enter", "respaldar la ruta seleccionada / restaurar un respaldo"},
		{"o / c", "restaurar en la ubicación original o en otra ruta"},
	}},
	{"Respaldo de configuraciones", [][2]string{
		{"a / d", "agregar una carpeta vigilada o dejar de vigilarla"},
		{"e / t", "editar o probar el repositorio"},
		{"r", "refrescar el historial"},
	}},
	{"Servicio", [][2]string{
		{"i / d / r", "iniciar, detener o reiniciar el servicio"},
		{"a", "actualizar el estado"},
		{"pgup/pgdn, fin", "desplazarse por los logs"},
	}},
}

// handleHelpUpdate handles key messages in the help screen
func (m Model) handleHelpUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "?", "enter":
		return m.back(menuView)
	}
	return m, nil
}

// renderHelp renders the keyboard shortcuts
func (m Model) renderHelp(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" AYUDA ") + "\n\n")

	for _, section := range helpSections {
		body.WriteString(headerStyle.Render(section.title) + "\n")
		for _, key := range section.keys {
			fmt.Fprintf(body, "  %s  %s\n", logTimeStyle.Render(fmt.Sprintf("%-15s", key[0])), key[1])
		}
		body.WriteString("\n")
	}

	body.WriteString(mutedStyle.Render("Los respaldos automáticos los ejecuta el servicio (baky daemon),\nno esta interfaz: puede cerrarla sin interrumpirlos.") + "\n")
	body.WriteString(footerStyle.Render("esc: volver"))
}
