package extractor

// Tag types that agents use in their daily logs
const (
	TagLearning = "[LEARNING]"
	TagUpdate   = "[UPDATE]"
	TagStale    = "[STALE]"
)

// Entry is a parsed learning from an agent's daily log.
type Entry struct {
	Tag     string // [LEARNING], [UPDATE], or [STALE]
	Topic   string // short identifier for matching
	Content string // the actual learning text
	Source  string // agent name + file path
}

// Extract parses [LEARNING], [UPDATE], [STALE] lines from markdown text.
// Format: [TAG] topic | content
func Extract(text string, agent, filePath string) []Entry {
	var entries []Entry
	lines := splitLines(text)

	for _, line := range lines {
		trimmed := trimSpace(line)
		// Strip markdown bullet prefixes: "- ", "* ", "1. ", etc.
		trimmed = stripBullet(trimmed)
		for _, tag := range []string{TagLearning, TagUpdate, TagStale} {
			if startsWith(trimmed, tag) {
				entry := parseTagLine(trimmed, tag, agent, filePath)
				if entry != nil {
					entries = append(entries, *entry)
				}
				break
			}
		}
	}
	return entries
}

func parseTagLine(line, tag, agent, filePath string) *Entry {
	// Strip the tag prefix
	rest := line[len(tag):]
	rest = trimSpace(rest)

	// Split on first " | " to get topic and content
	idx := indexOf(rest, " | ")
	if idx == -1 {
		// No pipe separator — treat entire rest as topic
		return &Entry{
			Tag:     tag,
			Topic:   trimSpace(rest),
			Content: "",
			Source:  agent + ":" + filePath,
		}
	}

	return &Entry{
		Tag:     tag,
		Topic:   trimSpace(rest[:idx]),
		Content: trimSpace(rest[idx+3:]),
		Source:  agent + ":" + filePath,
	}
}

// String helpers (no imports beyond stdlib needed)

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func stripBullet(s string) string {
	// "- text", "* text", "1. text", "2. text", etc.
	if len(s) >= 2 && (s[0] == '-' || s[0] == '*') && s[1] == ' ' {
		return trimSpace(s[2:])
	}
	// Ordered list: digits followed by ". "
	for i := 0; i < len(s) && i < 4; i++ {
		if s[i] >= '0' && s[i] <= '9' {
			continue
		}
		if s[i] == '.' && i > 0 && i+1 < len(s) && s[i+1] == ' ' {
			return trimSpace(s[i+2:])
		}
		break
	}
	return s
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
