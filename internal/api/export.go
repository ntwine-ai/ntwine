package api

import (
	"archive/zip"
	"fmt"
	"net/http"
	"strings"

	"github.com/ntwine-ai/ntwine/internal/config"
)

func handleExport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "discussion id is required")
		return
	}

	disc, err := config.LoadDiscussion(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.zip", disc.ID))

	zw := zip.NewWriter(w)
	defer zw.Close()

	writeZipFile(zw, "discussion.md", formatTranscript(disc))
	writeZipFile(zw, "notes.md", disc.SharedNotes)
	writeZipFile(zw, "execution-prompt.md", disc.ExecutionPrompt)
	writeZipFile(zw, "pinned.md", formatPinned(disc.PinnedMessages))
}

func writeZipFile(zw *zip.Writer, name string, content string) {
	f, err := zw.Create(name)
	if err != nil {
		return
	}
	f.Write([]byte(content))
}

func formatTranscript(disc config.DiscussionRecord) string {
	var sb strings.Builder
	sb.WriteString("# Discussion: ")
	sb.WriteString(disc.Prompt)
	sb.WriteString("\n\n")
	sb.WriteString("Models: ")
	sb.WriteString(strings.Join(disc.Models, ", "))
	sb.WriteString("\n\n---\n\n")

	for _, m := range disc.Messages {
		name := m.DisplayName
		if name == "" {
			name = m.ModelID
		}
		sb.WriteString("**")
		sb.WriteString(name)
		sb.WriteString("**: ")
		sb.WriteString(m.Content)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func formatPinned(pins []string) string {
	if len(pins) == 0 {
		return "No pinned messages."
	}
	var sb strings.Builder
	sb.WriteString("# Pinned Messages\n\n")
	for i, p := range pins {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, p))
	}
	return sb.String()
}
