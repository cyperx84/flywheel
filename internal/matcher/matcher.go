package matcher

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Match uses QMD to find the best matching vault note for a topic.
// Returns the note ID and relevance score, or empty string if no match.
func Match(qmdbin, topic, content string) (noteID string, score int, err error) {
	query := topic
	if content != "" {
		query = fmt.Sprintf("%s %s", topic, content)
	}

	cmd := exec.Command(qmdbin, "query", query, "--max-results", "1", "--json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", 0, fmt.Errorf("qmd query: %w\n%s", err, string(out))
	}

	// Parse QMD JSON output
	var results []qmdResult
	if err := json.Unmarshal(out, &results); err != nil {
		// Fallback: try parsing plain text output
		return parsePlainTextMatch(string(out))
	}

	if len(results) == 0 {
		return "", 0, nil
	}

	// Extract note ID from qmd://vault/<id>.md:line format
	uri := results[0].URI
	noteID = extractNoteID(uri)
	score = results[0].Score

	return noteID, score, nil
}

type qmdResult struct {
	URI   string `json:"uri"`
	Title string `json:"title"`
	Score int    `json:"score"`
}

func extractNoteID(uri string) string {
	// qmd://vault/note-id.md:25 → note-id
	uri = strings.TrimPrefix(uri, "qmd://vault/")
	uri = strings.TrimPrefix(uri, "vault/")
	idx := strings.Index(uri, ".md")
	if idx > 0 {
		return uri[:idx]
	}
	return uri
}

func parsePlainTextMatch(text string) (noteID string, score int, err error) {
	// Parse lines like: qmd://vault/note-id.md:25 #abc123
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "qmd://vault/") {
			parts := strings.SplitN(line, " ", 2)
			noteID = extractNoteID(parts[0])
			// Try to parse score from surrounding text
			if len(parts) > 1 && strings.Contains(parts[1], "Score:") {
				scoreStr := parts[1]
				fmt.Sscanf(scoreStr, "Score: %d", &score)
			}
			return noteID, score, nil
		}
	}
	return "", 0, nil
}

// IsAvailable checks if QMD binary exists and is executable.
func IsAvailable(qmdbin string) bool {
	cmd := exec.Command(qmdbin, "--help")
	return cmd.Run() == nil
}
