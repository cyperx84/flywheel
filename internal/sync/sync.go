package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cyperx84/flywheel/internal/extractor"
	"github.com/cyperx84/flywheel/internal/matcher"
	"github.com/cyperx84/flywheel/internal/vault"
)

type Options struct {
	Since  string
	Agent  string
	DryRun bool
	JSON   bool
	Agents []string
	Dir    string
	Vault  string
	QMDBin string
}

type Result struct {
	Created int      `json:"created"`
	Stale   int      `json:"stale"`
	Skipped int      `json:"skipped"`
	Errors  int      `json:"errors"`
	Details []Detail `json:"details,omitempty"`
}

type Detail struct {
	Action  string   `json:"action"`
	Topic   string   `json:"topic"`
	Agent   string   `json:"agent"`
	Content string   `json:"content,omitempty"`
	Links   []string `json:"links,omitempty"`
}

func Run(opts Options) error {
	agents := opts.Agents
	if opts.Agent != "" {
		agents = []string{opts.Agent}
	}

	qmdOK := matcher.IsAvailable(opts.QMDBin)

	// Collect all entries
	var allEntries []extractor.Entry
	for _, agent := range agents {
		entries := scanAgent(opts.Dir, agent, opts.Since)
		allEntries = append(allEntries, entries...)
	}

	if len(allEntries) == 0 {
		if opts.JSON {
			out, _ := json.Marshal(Result{})
			fmt.Println(string(out))
		} else {
			fmt.Printf("No learnings found since %s\n", opts.Since)
		}
		return nil
	}

	if qmdOK {
		fmt.Println("🔍 QMD available — will add related links")
	}
	fmt.Println()

	now := time.Now().Format("2006-01-02")
	result := Result{}

	for _, entry := range allEntries {
		agent := agentFromSource(entry.Source)
		detail := Detail{Topic: entry.Topic, Agent: agent, Content: entry.Content}

		switch entry.Tag {
		case extractor.TagStale:
			result.Stale++
			detail.Action = "STALE"
			fmt.Printf("⚠️  STALE   %s\n   %s\n   from: %s\n\n", entry.Topic, entry.Content, agent)

		case extractor.TagLearning, extractor.TagUpdate:
			// Skip if note already exists
			noteRef := "inbox/" + topicToID(entry.Topic)
			if vault.Exists(noteRef) {
				result.Skipped++
				detail.Action = "SKIPPED (exists)"
				result.Details = append(result.Details, detail)
				continue
			}

			// Find related notes for wikilinks
			var links []matcher.Link
			if qmdOK {
				links = matcher.Related(opts.QMDBin, entry.Topic, entry.Content, 3)
			}

			// Build wikilinks
			var wikilinks []string
			for _, l := range links {
				wikilinks = append(wikilinks, fmt.Sprintf("[[%s]]", l.NoteID))
			}

			if opts.DryRun {
				result.Created++
				detail.Action = "CREATE (dry-run)"
				detail.Links = wikilinks
				linkStr := ""
				if len(wikilinks) > 0 {
					linkStr = fmt.Sprintf("\n   links: %s", strings.Join(wikilinks, " "))
				}
				fmt.Printf("📄 CREATE  inbox/%s.md (dry-run)\n   %s%s\n   from: %s\n\n",
					topicToID(entry.Topic), entry.Content, linkStr, agent)
			} else {
				// Build note content
				body := entry.Content
				if len(wikilinks) > 0 {
					body += "\n\nRelated: " + strings.Join(wikilinks, " ")
				}

				tag := "learning"
				if entry.Tag == extractor.TagUpdate {
					tag = "update"
				}

				err := vault.Create(vault.Note{
					ID:       topicToID(entry.Topic),
					Title:    entry.Topic,
					Content:  body,
					Tags:     []string{tag, agent},
					Aliases:  []string{entry.Topic},
					Folder:   "inbox",
					Modified: now,
				})
				if err != nil {
					result.Errors++
					detail.Action = "ERROR"
					fmt.Printf("❌ ERROR   %s: %v\n\n", entry.Topic, err)
				} else {
					result.Created++
					detail.Action = "CREATED"
					detail.Links = wikilinks
					fmt.Printf("✅ CREATED inbox/%s.md\n   from: %s\n\n", topicToID(entry.Topic), agent)
				}
			}

		default:
			result.Skipped++
		}

		result.Details = append(result.Details, detail)
	}

	fmt.Printf("Summary: %d created, %d skipped, %d stale, %d errors\n",
		result.Created, result.Skipped, result.Stale, result.Errors)

	if opts.JSON {
		out, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(out))
	}

	return nil
}

func scanAgent(workDir, agent, since string) []extractor.Entry {
	memoryDir := filepath.Join(workDir, agent, "memory")
	var entries []extractor.Entry

	files, err := filepath.Glob(filepath.Join(memoryDir, "*.md"))
	if err != nil {
		return entries
	}

	for _, f := range files {
		base := filepath.Base(f)
		// Compare date prefix (YYYY-MM-DD) against since
		if len(base) >= 10 && base[:10] < since {
			continue
		}

		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}

		extracted := extractor.Extract(string(data), agent, base)
		entries = append(entries, extracted...)
	}

	return entries
}

func topicToID(topic string) string {
	id := strings.ToLower(topic)
	id = strings.ReplaceAll(id, " ", "-")
	// Remove chars that aren't alphanumeric, hyphens, or underscores
	var clean strings.Builder
	for _, c := range id {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			clean.WriteRune(c)
		}
	}
	result := clean.String()
	if len(result) > 60 {
		result = result[:60]
	}
	return result
}

func agentFromSource(source string) string {
	// source format: "agent:filename.md"
	idx := strings.Index(source, ":")
	if idx > 0 {
		return source[:idx]
	}
	return source
}
