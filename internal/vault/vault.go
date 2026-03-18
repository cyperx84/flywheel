package vault

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Note represents a vault note to create or update.
type Note struct {
	ID       string
	Title    string
	Content  string
	Tags     []string
	Aliases  []string
	Folder   string // optional subfolder (e.g., "claw", "ideas")
	Modified string // ISO datetime
}

// Create writes a new note to the vault using obsidian-cli.
func Create(note Note) error {
	tags := append([]string{"reference"}, note.Tags...)
	tagLines := make([]string, len(tags))
	for i, t := range tags {
		tagLines[i] = fmt.Sprintf("  - %s", t)
	}

	aliasLines := make([]string, len(note.Aliases))
	for i, a := range note.Aliases {
		aliasLines[i] = fmt.Sprintf("  - %s", a)
	}

	content := fmt.Sprintf(`---
id: %s
title: %s
created: %s
modified: %s
tags:
%s
topics: []
refs: []
aliases:
%s
---

# %s

%s
`,
		note.ID,
		note.Title,
		note.Modified,
		note.Modified,
		strings.Join(tagLines, "\n"),
		strings.Join(aliasLines, "\n"),
		note.Title,
		note.Content,
	)

	args := []string{"create", note.ID, "--overwrite", "--content", content}
	if note.Folder != "" {
		args = append(args, "--folder", note.Folder)
	}

	cmd := exec.Command("obsidian-cli", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("obsidian-cli create: %w\n%s", err, string(out))
	}
	return nil
}

// UpdateFrontmatter sets a frontmatter field on an existing note.
func UpdateFrontmatter(noteID, key, value string) error {
	cmd := exec.Command("obsidian-cli", "frontmatter", noteID, "--edit", "--key", key, "--value", value)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("obsidian-cli frontmatter: %w\n%s", err, string(out))
	}
	return nil
}

// AppendContent appends text to an existing vault note by direct file edit.
// Per vault constitution, direct edits to existing note content are allowed.
func AppendContent(noteID, content string) error {
	// Use obsidian-cli to find the note path, then append
	cmd := exec.Command("obsidian-cli", "search-content", noteID)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("finding note %s: %w", noteID, err)
	}

	// Get the first matching file path
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return fmt.Errorf("note %s not found", noteID)
	}

	// The path from obsidian-cli — append content directly
	notePath := strings.TrimSpace(lines[0])

	f, err := openFileAppend(notePath)
	if err != nil {
		return fmt.Errorf("opening note %s: %w", noteID, err)
	}
	defer f.Close()

	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("appending to note %s: %w", noteID, err)
	}

	return nil
}

func openFileAppend(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
}

// Search finds notes matching a query via obsidian-cli.
func Search(query string) (string, error) {
	cmd := exec.Command("obsidian-cli", "search-content", query)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("obsidian-cli search: %w\n%s", err, string(out))
	}
	return string(out), nil
}
