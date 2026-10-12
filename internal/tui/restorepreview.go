package tui

import (
	"fmt"
	"strings"

	"github.com/Josedzzz/baky/internal/restore"
	"github.com/Josedzzz/baky/internal/textdiff"
	tea "github.com/charmbracelet/bubbletea"
)

type restorePreviewLoadedMsg struct {
	preview *restore.Preview
	err     error
}

type fileDiffLoadedMsg struct {
	lines []textdiff.Line
	err   error
}

// loadRestorePreviewCmd compares the backup with the restore destination
func loadRestorePreviewCmd(backupPath, destPath string) tea.Cmd {
	return func() tea.Msg {
		preview, err := restore.NewPreview(backupPath, destPath)
		return restorePreviewLoadedMsg{preview: preview, err: err}
	}
}

// loadFileDiffCmd compares the current file with the one in the backup
func loadFileDiffCmd(change restore.FileChange) tea.Cmd {
	return func() tea.Msg {
		lines, err := textdiff.Files(change.DestPath, change.BackupPath)
		return fileDiffLoadedMsg{lines: lines, err: err}
	}
}

// startRestorePreview opens the preview of the selected backup against
// the chosen destination
func (m Model) startRestorePreview() (tea.Model, tea.Cmd) {
	m.closeRestorePreview()
	m.state = restorePreviewView
	m.restorePreviewLoading = true
	m.restorePreviewErr = ""
	m.previewCursor = 0
	m.previewOffset = 0
	m.restoreActionIndex = 0
	m.message = ""
	return m, loadRestorePreviewCmd(m.selectedBackup.FullPath, m.restorePath)
}

// closeRestorePreview removes the temporary copy of the backup being previewed
func (m *Model) closeRestorePreview() {
	m.restorePreview.Close()
	m.restorePreview = nil
}

// visiblePreviewFiles is how many files of the preview fit on the screen
func (m Model) visiblePreviewFiles() int {
	visible := m.height - 18
	if visible < 3 {
		visible = 3
	}
	if visible > 12 {
		visible = 12
	}
	return visible
}

// handleRestorePreviewLoaded shows the preview once the backup was compared
func (m Model) handleRestorePreviewLoaded(msg restorePreviewLoadedMsg) (tea.Model, tea.Cmd) {
	// The user left the preview while it was loading: discard the result
	if m.state != restorePreviewView || !m.restorePreviewLoading {
		msg.preview.Close()
		return m, nil
	}

	m.restorePreviewLoading = false
	if msg.err != nil {
		m.restorePreviewErr = msg.err.Error()
		return m, nil
	}
	m.restorePreview = msg.preview
	return m, nil
}

// handleRestorePreviewUpdate handles key messages in the list of changes
func (m Model) handleRestorePreviewUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "esc" {
		m.closeRestorePreview()
		m.restorePreviewLoading = false
		m.state = selectRestoreDestView
		m.message = ""
		return m, nil
	}
	if m.restorePreview == nil {
		return m, nil
	}

	files := m.restorePreview.Files
	visible := m.visiblePreviewFiles()

	switch msg.String() {
	case "up", "k":
		m.previewCursor--
	case "down", "j":
		m.previewCursor++
	case "pgup":
		m.previewCursor -= visible
	case "pgdown":
		m.previewCursor += visible
	case "enter":
		if len(files) == 0 {
			return m, nil
		}
		// Compare the selected file line by line
		m.state = restoreFileDiffView
		m.fileDiffLoading = true
		m.fileDiffErr = ""
		m.fileDiffEmpty = false
		return m, loadFileDiffCmd(files[m.previewCursor])
	case "r":
		// Continue with the restore: choose what to do with existing files
		m.state = selectRestoreActionView
		m.restoreActionIndex = 0
		m.message = ""
		return m, nil
	}

	// Keep the cursor inside the list and the list around the cursor
	if m.previewCursor > len(files)-1 {
		m.previewCursor = len(files) - 1
	}
	if m.previewCursor < 0 {
		m.previewCursor = 0
	}
	if m.previewCursor < m.previewOffset {
		m.previewOffset = m.previewCursor
	}
	if m.previewCursor >= m.previewOffset+visible {
		m.previewOffset = m.previewCursor - visible + 1
	}
	return m, nil
}

// handleFileDiffLoaded shows the comparison of one file
func (m Model) handleFileDiffLoaded(msg fileDiffLoadedMsg) (tea.Model, tea.Cmd) {
	m.fileDiffLoading = false
	if msg.err != nil {
		m.fileDiffErr = msg.err.Error()
		return m, nil
	}
	m.fileDiffEmpty = len(msg.lines) == 0
	m.diffView.SetContent(renderDiffLines(msg.lines))
	m.diffView.GotoTop()
	return m, nil
}

// handleRestoreFileDiffUpdate handles key messages in the comparison of one file
func (m Model) handleRestoreFileDiffUpdate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.state = restorePreviewView
		return m, nil
	}
	var cmd tea.Cmd
	m.diffView, cmd = m.diffView.Update(msg)
	return m, cmd
}

// changeLabel returns the mark and the description of a kind of change
func changeLabel(kind restore.ChangeKind) (mark, description string) {
	switch kind {
	case restore.ChangeModified:
		return logWarnStyle.Render("M"), "se modifica"
	case restore.ChangeNew:
		return diffAddedStyle.Render("+"), "se crea"
	default:
		return mutedStyle.Render("="), "queda igual"
	}
}

// renderRestorePreview renders what restoring the backup would change
func (m Model) renderRestorePreview(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" CAMBIOS DE LA RESTAURACIÓN ") + "\n\n")
	body.WriteString(headerStyle.Render("Respaldo: ") + m.selectedBackup.Filename + "\n")
	body.WriteString(headerStyle.Render("Destino:  ") + m.restorePath + "\n\n")

	switch {
	case m.restorePreviewLoading:
		body.WriteString(processingStyle.Render("Comparando el respaldo con el destino...") + "\n")
		body.WriteString(footerStyle.Render("esc: volver"))
		return
	case m.restorePreviewErr != "":
		body.WriteString(errorStyle.Render("No se pudo revisar el respaldo: "+m.restorePreviewErr) + "\n")
		body.WriteString(footerStyle.Render("esc: volver"))
		return
	case m.restorePreview == nil:
		return
	}

	p := m.restorePreview
	body.WriteString(logWarnStyle.Render(fmt.Sprintf("M se modifican: %d", p.Modified)) + "   " +
		diffAddedStyle.Render(fmt.Sprintf("+ se crean: %d", p.New)) + "   " +
		mutedStyle.Render(fmt.Sprintf("= quedan igual: %d", p.Same)) + "\n")
	if p.OnlyInDest > 0 {
		body.WriteString(mutedStyle.Render(fmt.Sprintf("Archivos del destino que no están en el respaldo: %d (\"Sobrescribir\" los conserva).", p.OnlyInDest)) + "\n")
	}
	if !p.HasChanges() {
		body.WriteString(successStyle.Render("El destino ya es igual al respaldo: no hay nada que restaurar.") + "\n")
	}
	body.WriteString("\n")

	visible := m.visiblePreviewFiles()
	for i := m.previewOffset; i < len(p.Files) && i < m.previewOffset+visible; i++ {
		f := p.Files[i]
		mark, description := changeLabel(f.Kind)
		line := fmt.Sprintf("%s  (%s)", f.Path, description)
		if m.previewCursor == i {
			body.WriteString(mark + selectedItemStyle.Render(line) + "\n")
		} else {
			body.WriteString(mark + itemStyle.Render(line) + "\n")
		}
	}
	if len(p.Files) > visible {
		body.WriteString(statusStyle.Render(fmt.Sprintf("[Archivo %d de %d]", m.previewCursor+1, len(p.Files))) + "\n")
	}

	m.renderMessage(body)
	body.WriteString(footerStyle.Render("\n↑/↓: seleccionar • enter: ver diferencias • r: continuar con la restauración • esc: volver"))
}

// renderRestoreFileDiff renders the comparison between the current file
// and the one in the backup
func (m Model) renderRestoreFileDiff(body *strings.Builder) {
	body.WriteString(titleStyle.Render(" DIFERENCIAS DEL ARCHIVO ") + "\n\n")

	if m.restorePreview == nil || m.previewCursor >= len(m.restorePreview.Files) {
		return
	}
	change := m.restorePreview.Files[m.previewCursor]
	_, description := changeLabel(change.Kind)
	body.WriteString(headerStyle.Render("Archivo:  ") + change.DestPath + "\n")
	body.WriteString(headerStyle.Render("Respaldo: ") + m.selectedBackup.Filename + mutedStyle.Render("  ("+description+")") + "\n\n")

	footer := "esc: volver"
	switch {
	case m.fileDiffLoading:
		body.WriteString(processingStyle.Render("Comparando...") + "\n")
	case m.fileDiffErr != "":
		body.WriteString(errorStyle.Render(m.fileDiffErr) + "\n")
	case change.Kind == restore.ChangeSame || m.fileDiffEmpty:
		body.WriteString(successStyle.Render("El archivo actual ya es igual al del respaldo.") + "\n")
	default:
		if change.Kind == restore.ChangeNew {
			body.WriteString(mutedStyle.Render("El archivo no existe en el destino: restaurar lo crearía.") + "\n")
		}
		body.WriteString("Si restaura el respaldo:  " +
			diffRemovedStyle.Render("- línea actual que se quita") + "   " +
			diffAddedStyle.Render("+ línea que vuelve") + "\n\n")
		body.WriteString(m.diffView.View() + "\n")
		body.WriteString(statusStyle.Render(fmt.Sprintf("[%d%%]", int(m.diffView.ScrollPercent()*100))) + "\n")
		footer = "↑/↓: desplazar • pgup/pgdn: página • esc: volver"
	}

	body.WriteString(footerStyle.Render(footer))
}
