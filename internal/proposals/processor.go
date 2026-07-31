package proposals

import (
	"context"
	"fmt"
	"log"
	"os"
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

		f, err := os.OpenFile(p.journalPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("journal: %w", err)
		}
		defer f.Close()

		if _, err := f.WriteString(entry); err != nil {
			return fmt.Errorf("journal: write: %w", err)
		}
	}

	if prop.Summary != "" {
		log.Printf("[reflection] %s: %s", prop.ReflectionID, prop.Summary)
	}

	return nil
}
