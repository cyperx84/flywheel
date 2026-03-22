package matcher

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Related returns up to maxLinks vault notes related to a topic+content query.
// Used for wikilinks, not for deciding where to write.
func Related(qmdBin, topic, content string, maxLinks int) []Link {
	if maxLinks <= 0 {
		maxLinks = 3
	}

	query := topic
	if content != "" {
		query = fmt.Sprintf("%s %s", topic, content)
	}

	cmd := exec.Command(qmdBin, "query", query, "--max-results", fmt.Sprintf("%d", maxLinks), "--json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}

	// QMD prints warnings before JSON — find the opening bracket
	raw := string(out)
	jsonStart := strings.Index(raw, "[")
	if jsonStart < 0 {
		return nil
	}

	var results []qmdResult
	if err := json.Unmarshal([]byte(raw[jsonStart:]), &results); err != nil {
		return nil
	}

	var links []Link
	for _, r := range results {
		if r.Score < 0.40 {
			continue // skip very weak matches
		}
		noteID := extractNoteID(r.File)
		if noteID != "" {
			links = append(links, Link{
				NoteID: noteID,
				Title:  r.Title,
				Score:  int(r.Score * 100),
			})
		}
	}
	return links
}

// Link is a related vault note.
type Link struct {
	NoteID string
	Title  string
	Score  int
}

type qmdResult struct {
	DocID   string  `json:"docid"`
	File    string  `json:"file"`
	Title   string  `json:"title"`
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet"`
}

func extractNoteID(uri string) string {
	// qmd://vault/claw/note-id.md → claw/note-id
	uri = strings.TrimPrefix(uri, "qmd://vault/")
	idx := strings.Index(uri, ".md")
	if idx > 0 {
		return uri[:idx]
	}
	return uri
}

// IsAvailable checks if QMD binary exists and is executable.
func IsAvailable(qmdBin string) bool {
	cmd := exec.Command(qmdBin, "--help")
	return cmd.Run() == nil
}
