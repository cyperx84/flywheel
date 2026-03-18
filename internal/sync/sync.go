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
	Updated   int      `json:"updated"`
	Created   int      `json:"created"`
	Matched   int      `json:"matched"`
	Stale     int      `json:"stale"`
	Skipped   int      `json:"skipped"`
	Errors    int      `json:"errors"`
	Details   []Detail `json:"details,omitempty"`
	QMDEnabled bool    `json:"qmd_enabled"`
}

type Detail struct {
	Action string `json:"action"`
	Topic  string `json:"topic"`
	Note   string `json:"note,omitempty"`
	Source string `json:"source"`
	Score  int    `json:"score,omitempty"`
}

func Run(opts Options) error {
	agents := opts.Agents
	if opts.Agent != "" {
		agents = []string{opts.Agent}
	}

	qmdAvailable := matcher.IsAvailable(opts.QMDBin)

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
			if !qmdAvailable {
				fmt.Println("⚠️  QMD not available — using topic ID matching only")
			}
		}
		return nil
	}

	result := Result{QMDEnabled: qmdAvailable}
	now := time.Now().Format("2006-01-02 15:04")

	if qmdAvailable {
		fmt.Println("🔍 QMD matching enabled")
	} else {
		fmt.Println("📝 Topic ID matching (QMD unavailable)")
	}
	fmt.Println()

	for _, entry := range allEntries {
		detail := Detail{Topic: entry.Topic, Source: entry.Source}

		switch entry.Tag {
		case extractor.TagStale:
			result.Stale++
			detail.Action = "STALE"
			fmt.Printf("⚠️  STALE   %s\n   %s\n   Via: %s\n\n", entry.Topic, entry.Content, entry.Source)

		case extractor.TagLearning:
			// Try QMD match first, fall back to topic ID
			noteID := ""
			score := 0
			if qmdAvailable {
				noteID, score, _ = matcher.Match(opts.QMDBin, entry.Topic, entry.Content)
			}
			if noteID == "" {
				noteID = topicToID(entry.Topic)
			}

			if opts.DryRun {
				if score > 0 {
					result.Matched++
					detail.Action = "MATCHED (dry-run)"
					detail.Note = noteID
					detail.Score = score
					fmt.Printf("🔗 MATCH   %s → %s (score: %d, dry-run)\n   %s\n\n", entry.Topic, noteID, score, entry.Content)
				} else {
					result.Created++
					detail.Action = "CREATED (dry-run)"
					detail.Note = noteID
					fmt.Printf("📄 CREATE  %s (dry-run)\n   %s\n\n", entry.Topic, entry.Content)
				}
			} else if score > 0 {
				// Existing note found — update it
				err := vault.UpdateFrontmatter(noteID, "modified", now)
				if err != nil {
					// Try create fallback
					err2 := createNote(noteID, entry, now)
					if err2 != nil {
						result.Errors++
						detail.Action = "ERROR"
						fmt.Printf("❌ ERROR   %s: %v\n\n", entry.Topic, err2)
					} else {
						result.Created++
						detail.Action = "CREATED"
						detail.Note = noteID
						fmt.Printf("✅ CREATED %s (QMD matched but note missing, created)\n   Via: %s\n\n", entry.Topic, entry.Source)
					}
				} else {
					result.Matched++
					result.Updated++
					detail.Action = "MATCHED"
					detail.Note = noteID
					detail.Score = score
					fmt.Printf("🔗 MATCHED %s → %s (score: %d)\n   Via: %s\n\n", entry.Topic, noteID, score, entry.Source)
				}
			} else {
				err := createNote(noteID, entry, now)
				if err != nil {
					result.Errors++
					detail.Action = "ERROR"
					fmt.Printf("❌ ERROR   %s: %v\n\n", entry.Topic, err)
				} else {
					result.Created++
					detail.Action = "CREATED"
					detail.Note = noteID
					fmt.Printf("✅ CREATED %s\n   %s\n   Via: %s\n\n", entry.Topic, entry.Content, entry.Source)
				}
			}

		case extractor.TagUpdate:
			noteID := ""
			score := 0
			if qmdAvailable {
				noteID, score, _ = matcher.Match(opts.QMDBin, entry.Topic, entry.Content)
			}
			if noteID == "" {
				noteID = topicToID(entry.Topic)
			}

			if opts.DryRun {
				if score > 0 {
					result.Matched++
					result.Updated++
					detail.Action = "UPDATED (dry-run)"
					detail.Note = noteID
					detail.Score = score
					fmt.Printf("✏️  UPDATE  %s → %s (score: %d, dry-run)\n\n", entry.Topic, noteID, score)
				} else {
					result.Created++
					detail.Action = "CREATED (dry-run)"
					detail.Note = noteID
					fmt.Printf("📄 CREATE  %s (dry-run)\n   %s\n\n", entry.Topic, entry.Content)
				}
			} else if score > 0 {
				err := vault.UpdateFrontmatter(noteID, "modified", now)
				if err != nil {
					err2 := createNote(noteID, entry, now)
					if err2 != nil {
						result.Errors++
						detail.Action = "ERROR"
						fmt.Printf("❌ ERROR   %s: %v\n\n", entry.Topic, err2)
					} else {
						result.Created++
						detail.Action = "CREATED"
						detail.Note = noteID
						fmt.Printf("✅ CREATED %s (update target not found, created)\n   Via: %s\n\n", entry.Topic, entry.Source)
					}
				} else {
					result.Matched++
					result.Updated++
					detail.Action = "UPDATED"
					detail.Note = noteID
					detail.Score = score
					fmt.Printf("✅ UPDATED %s → %s (score: %d)\n   Via: %s\n\n", entry.Topic, noteID, score, entry.Source)
				}
			} else {
				err := createNote(noteID, entry, now)
				if err != nil {
					result.Errors++
					detail.Action = "ERROR"
					fmt.Printf("❌ ERROR   %s: %v\n\n", entry.Topic, err)
				} else {
					result.Created++
					detail.Action = "CREATED"
					detail.Note = noteID
					fmt.Printf("✅ CREATED %s (no match, created new)\n   %s\n   Via: %s\n\n", entry.Topic, entry.Content, entry.Source)
				}
			}

		default:
			result.Skipped++
		}

		result.Details = append(result.Details, detail)
	}

	fmt.Printf("Summary: %d matched, %d updated, %d created, %d stale, %d errors\n",
		result.Matched, result.Updated, result.Created, result.Stale, result.Errors)

	if opts.JSON {
		out, _ := json.Marshal(result)
		fmt.Println(string(out))
	}

	return nil
}

func createNote(noteID string, entry extractor.Entry, now string) error {
	return vault.Create(vault.Note{
		ID:       noteID,
		Title:    entry.Topic,
		Content:  entry.Content,
		Tags:     []string{},
		Aliases:  []string{entry.Topic},
		Modified: now,
	})
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
	if len(id) > 60 {
		id = id[:60]
	}
	return id
}
