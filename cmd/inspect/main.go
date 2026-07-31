package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Espectro0/AuroraProject/config"
	embednvidia "github.com/Espectro0/AuroraProject/internal/embedder/nvidia"
	"github.com/Espectro0/AuroraProject/internal/memory"
	"github.com/Espectro0/AuroraProject/internal/memory/chromem"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	emb := embednvidia.New(cfg.NvidiaApiKey, cfg.EmbedderModel, cfg.EmbedderBaseURL, 30)
	store, err := chromem.NewStore("aurora", emb)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()

	nodes, err := store.SearchNodes(ctx, "memorias", math.MaxInt)
	if err != nil {
		log.Fatal(err)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].CreatedAt.Before(nodes[j].CreatedAt)
	})

	fmt.Println("Aurora — Estado de la memoria")
	fmt.Println("════════════════════════════════")

	fmt.Printf("\nNODOS (%d)\n", len(nodes))
	for i, n := range nodes {
		fmt.Printf("  [%d] %-9s %s\n", i+1, n.Type, labelOf(n))
		fmt.Printf("      contenido: %q\n", n.Content)
		fmt.Printf("      creado:    %s\n", n.CreatedAt.Format(time.RFC3339))
	}

	fmt.Printf("\nARISTAS (%d)\n", countEdges())
	byID := make(map[string]memory.Node, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	printEdges(byID)

	journalStatus()
}

func labelOf(n memory.Node) string {
	if idx := strings.Index(n.Content, ":"); idx > 0 {
		return strings.TrimSpace(n.Content[:idx])
	}
	return n.Content
}

func countEdges() int {
	ef := loadEdges()
	n := 0
	for _, edges := range ef {
		n += len(edges)
	}
	return n
}

func printEdges(byID map[string]memory.Node) {
	ef := loadEdges()
	if len(ef) == 0 {
		return
	}

	srcIDs := make([]string, 0, len(ef))
	for id := range ef {
		srcIDs = append(srcIDs, id)
	}
	sort.Strings(srcIDs)

	for _, srcID := range srcIDs {
		src := displayName(srcID, byID)
		for _, e := range ef[srcID] {
			dst := displayName(e.TargetID, byID)
			fmt.Printf("  [%s] --%s--> [%s]\n", src, e.Type, dst)
		}
	}
}

func displayName(id string, byID map[string]memory.Node) string {
	if n, ok := byID[id]; ok {
		return labelOf(n)
	}
	return id
}

func loadEdges() map[string][]memory.Edge {
	raw, err := os.ReadFile("aurora.edges.json")
	if err != nil {
		return nil
	}

	var ef map[string][]memory.Edge
	if err := json.Unmarshal(raw, &ef); err != nil {
		return nil
	}

	return ef
}

func journalStatus() {
	fi, err := os.Stat("journal.md")
	if err != nil {
		fmt.Println("\nJOURNAL (journal.md) — no existe")
		return
	}

	data, err := os.ReadFile("journal.md")
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		fmt.Printf("\nJOURNAL (journal.md) — %d bytes, vacio\n", fi.Size())
		return
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	start := len(lines) - 3
	if start < 0 {
		start = 0
	}

	fmt.Printf("\nJOURNAL (journal.md) — %d bytes, ultimas entradas:\n", fi.Size())
	for _, l := range lines[start:] {
		fmt.Printf("  %s\n", l)
	}
}
