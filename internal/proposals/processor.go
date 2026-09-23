package proposals

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

type SimpleProcessor struct {
	journalPath string
}

func NewSimpleProcessor(journalPath string) *SimpleProcessor {
	return &SimpleProcessor{journalPath: journalPath}
}

func (p *SimpleProcessor) Process(ctx context.Context, prop Proposal) error {
	if prop.Journal != nil {
		entry := fmt.Sprintf("\n## %s\n\n%s\n\n*Estado: %s*\n",
			prop.Timestamp.Format(time.RFC3339),
			prop.Journal.Content,
			prop.Journal.Mood,
		)

		if err := p.appendJournal(entry); err != nil {
			return err
		}
	}

	if prop.Summary != "" {
		log.Printf("[reflection] %s: %s", prop.ReflectionID, prop.Summary)
	}

	return nil
}

func (p *SimpleProcessor) AppendNote(note string) error {
	return p.appendJournal("\n" + note + "\n")
}

func (p *SimpleProcessor) appendJournal(text string) error {
	f, err := os.OpenFile(p.journalPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("journal: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(text); err != nil {
		return fmt.Errorf("journal: write: %w", err)
	}
	return nil
}

type JournalEntry struct {
	TS      time.Time `json:"timestamp"`
	Content string    `json:"content"`
	Mood    string    `json:"mood"`
}

var (
	journalHeaderRe = regexp.MustCompile(`^## (.+)$`)
	journalMoodRe   = regexp.MustCompile(`^\*Estado: (.+)\*$`)
)

func (p *SimpleProcessor) Read() ([]JournalEntry, error) {
	f, err := os.Open(p.journalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []JournalEntry{}, nil
		}
		return nil, fmt.Errorf("journal: read: %w", err)
	}
	defer f.Close()

	var entries []JournalEntry
	var current *JournalEntry
	var contentLines []string

	flush := func() {
		if current != nil {
			current.Content = strings.TrimSpace(strings.Join(contentLines, "\n"))
			entries = append(entries, *current)
		}
		contentLines = nil
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if m := journalHeaderRe.FindStringSubmatch(line); m != nil {
			flush()
			ts, _ := time.Parse(time.RFC3339, strings.TrimSpace(m[1]))
			current = &JournalEntry{TS: ts}
			continue
		}

		if m := journalMoodRe.FindStringSubmatch(line); m != nil && current != nil {
			current.Mood = strings.TrimSpace(m[1])
			continue
		}

		if current != nil {
			contentLines = append(contentLines, line)
		}
	}
	flush()

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("journal: scan: %w", err)
	}

	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	return entries, nil
}
