package session

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/benlutz/preflt/internal/checklist"
	"github.com/benlutz/preflt/internal/format"
	"github.com/benlutz/preflt/internal/store"
)

// Resume finds a resumable run state for the given checklist, prompting the
// user via stdin if one is found. If the checklist has changed since the last
// session (item IDs no longer match in order), the stale state is discarded
// automatically and a fresh run starts. Returns the state to use — either the
// existing one or a new one.
func Resume(name, path string, flat []checklist.FlatItem) (*store.RunState, error) {
	existing, err := store.LoadLatestState(name)
	if err != nil {
		return nil, fmt.Errorf("checking for existing session: %w", err)
	}

	if existing != nil && !itemIDsMatch(existing.Items, flat) {
		fmt.Println("  Checklist has changed since last session. Starting fresh.")
		_ = store.DiscardState(existing)
		existing = nil
	}

	if existing != nil {
		ago := time.Since(existing.UpdatedAt).Round(time.Second)
		fmt.Printf("  Session found (%s ago). Resume? [y/n] ", format.Duration(ago))
		reader := bufio.NewReader(os.Stdin)
		resp, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(resp)) == "y" {
			return existing, nil
		}
	}

	itemIDs := make([]string, len(flat))
	for i, fi := range flat {
		itemIDs[i] = fi.Item.ID
	}
	return store.NewRunState(name, path, itemIDs), nil
}

// itemIDsMatch returns true when the persisted item IDs exactly match the
// current flat checklist in the same order. A mismatch means the checklist
// was edited and the saved session is stale.
func itemIDsMatch(items []store.ItemState, flat []checklist.FlatItem) bool {
	if len(items) != len(flat) {
		return false
	}
	for i, fi := range flat {
		if items[i].ID != fi.Item.ID {
			return false
		}
	}
	return true
}
