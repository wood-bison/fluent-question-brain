// Command qb-release promotes an explicitly approved vault snapshot into the
// published Question Brain projection. Ordinary imports remain draft-only;
// this command is the deliberate content cutover boundary.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wood-bison/fluent-question-brain/internal/normalize"
	"github.com/wood-bison/fluent-question-brain/internal/store"
)

var cardDirectories = map[string]bool{
	"Question Cards":      true,
	"Concept Cards":       true,
	"Best Practice Cards": true,
	"Behavioral Cards":    true,
}

type releaseItem struct {
	SourceRef   string `json:"source_ref"`
	StableKey   string `json:"stable_key,omitempty"`
	ContentHash string `json:"content_hash,omitempty"`
	Action      string `json:"action"`
	QuestionID  string `json:"question_id,omitempty"`
	Error       string `json:"error,omitempty"`
}

type releaseReport struct {
	ContractVersion string         `json:"contract_version"`
	SourceRoot      string         `json:"source_root"`
	WorkspaceKey    string         `json:"workspace_key"`
	Approved        bool           `json:"approved"`
	StartedAt       time.Time      `json:"started_at"`
	FinishedAt      time.Time      `json:"finished_at"`
	Totals          map[string]int `json:"totals"`
	DuplicateKeys   []string       `json:"duplicate_stable_keys,omitempty"`
	DuplicateHashes []string       `json:"duplicate_content_hashes,omitempty"`
	Items           []releaseItem  `json:"items"`
}

func main() {
	root := flag.String("root", "", "vault root containing the four canonical card directories")
	databaseURL := flag.String("database-url", os.Getenv("DATABASE_URL"), "Postgres URL; required for an approved release")
	workspaceKey := flag.String("workspace-key", "fluent-interview", "workspace stable key")
	workspaceName := flag.String("workspace-name", "Fluent Interview", "workspace display name")
	actor := flag.String("actor", "vault-release", "audit actor")
	reportPath := flag.String("report", "", "optional JSON report path")
	approve := flag.Bool("approve-source-vault", false, "publish this immutable source snapshot; without this flag the command is dry-run only")
	flag.Parse()

	if strings.TrimSpace(*root) == "" {
		fail("-root is required")
	}
	absoluteRoot, err := filepath.Abs(*root)
	if err != nil {
		fail("resolve root: %v", err)
	}
	files, err := collectFiles(absoluteRoot)
	if err != nil {
		fail("collect cards: %v", err)
	}
	report := releaseReport{
		ContractVersion: "question-brain.vault-release.v1",
		SourceRoot:      absoluteRoot,
		WorkspaceKey:    *workspaceKey,
		Approved:        *approve,
		StartedAt:       time.Now().UTC(),
		Totals:          map[string]int{"files": len(files)},
		Items:           make([]releaseItem, 0, len(files)),
	}

	cards := make([]normalize.Card, 0, len(files))
	byKey := make(map[string][]string)
	byHash := make(map[string][]string)
	for _, file := range files {
		item := releaseItem{SourceRef: file}
		content, readErr := os.ReadFile(file)
		if readErr != nil {
			item.Action = "invalid"
			item.Error = readErr.Error()
			report.Items = append(report.Items, item)
			report.Totals[item.Action]++
			continue
		}
		card, parseErr := normalize.ParseMarkdown(file, content)
		if parseErr != nil {
			item.Action = "invalid"
			item.Error = parseErr.Error()
			report.Items = append(report.Items, item)
			report.Totals[item.Action]++
			continue
		}
		prompt, _, _ := normalize.EnglishFields(card)
		hasBody := false
		for _, section := range card.Sections {
			if strings.TrimSpace(section.Body) != "" {
				hasBody = true
				break
			}
		}
		if strings.TrimSpace(prompt) == "" || !hasBody {
			item.Action = "invalid"
			item.StableKey = card.StableKey
			item.ContentHash = card.Hash
			item.Error = "card must contain a prompt and at least one non-empty section"
			report.Items = append(report.Items, item)
			report.Totals[item.Action]++
			continue
		}
		item.StableKey = card.StableKey
		item.ContentHash = card.Hash
		item.Action = "validated"
		report.Items = append(report.Items, item)
		report.Totals[item.Action]++
		cards = append(cards, card)
		byKey[card.StableKey] = append(byKey[card.StableKey], file)
		byHash[card.Hash] = append(byHash[card.Hash], file)
	}

	report.DuplicateKeys = duplicateGroups(byKey)
	report.DuplicateHashes = duplicateGroups(byHash)
	if len(report.DuplicateKeys) > 0 || len(report.DuplicateHashes) > 0 {
		for _, key := range report.DuplicateKeys {
			report.Totals["duplicate_stable_key"]++
			_ = key
		}
		for _, hash := range report.DuplicateHashes {
			report.Totals["duplicate_content_hash"]++
			_ = hash
		}
		if *approve {
			failReport(&report, "release refused: duplicate stable keys or content hashes require an explicit review")
		}
	}
	if *approve && report.Totals["invalid"] > 0 {
		failReport(&report, "release refused: invalid cards remain in the source snapshot")
	}

	if !*approve {
		report.Totals["would_publish"] = len(cards)
	} else {
		if strings.TrimSpace(*databaseURL) == "" {
			failReport(&report, "-database-url or DATABASE_URL is required for an approved release")
		}
		db, openErr := store.Open(context.Background(), *databaseURL)
		if openErr != nil {
			failReport(&report, "open database: %v", openErr)
		}
		defer db.Close()
		for index, card := range cards {
			stored, publishErr := db.PublishImportedCard(context.Background(), card, *workspaceKey, *workspaceName, *actor)
			if publishErr != nil {
				report.Items[indexFor(report.Items, card.SourceRef)].Action = "failed"
				report.Items[indexFor(report.Items, card.SourceRef)].Error = publishErr.Error()
				report.Totals["failed"]++
				failReport(&report, "publish %s: %v", card.StableKey, publishErr)
			}
			itemIndex := indexFor(report.Items, card.SourceRef)
			report.Items[itemIndex].Action = stored.Action
			report.Items[itemIndex].QuestionID = stored.QuestionID
			report.Totals["published"]++
			if index > 0 && index%100 == 0 {
				fmt.Printf("published %d/%d\n", index, len(cards))
			}
		}
	}
	report.FinishedAt = time.Now().UTC()
	if *reportPath != "" {
		encoded, encodeErr := json.MarshalIndent(report, "", "  ")
		if encodeErr != nil {
			fail("encode report: %v", encodeErr)
		}
		if writeErr := os.WriteFile(*reportPath, append(encoded, '\n'), 0o644); writeErr != nil {
			fail("write report: %v", writeErr)
		}
	}
	encoded, _ := json.Marshal(report.Totals)
	fmt.Printf("vault release %s: %s\n", map[bool]string{true: "published", false: "dry-run"}[*approve], encoded)
}

func collectFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) == 0 || !cardDirectories[parts[0]] {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func duplicateGroups(groups map[string][]string) []string {
	keys := make([]string, 0)
	for key, files := range groups {
		if len(files) > 1 {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func indexFor(items []releaseItem, sourceRef string) int {
	for index := range items {
		if items[index].SourceRef == sourceRef {
			return index
		}
	}
	return -1
}

func failReport(report *releaseReport, format string, args ...any) {
	report.FinishedAt = time.Now().UTC()
	encoded, _ := json.Marshal(report.Totals)
	fmt.Fprintf(os.Stderr, "vault release refused: "+format+"\nsummary=%s\n", append(args, encoded)...)
	os.Exit(1)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}
