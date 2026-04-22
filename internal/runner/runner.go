package runner

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
	"github.com/benlutz/preflt/internal/automation"
	"github.com/benlutz/preflt/internal/checklist"
	"github.com/benlutz/preflt/internal/session"
	"github.com/benlutz/preflt/internal/store"
	"github.com/benlutz/preflt/internal/tui"
)

// Run starts a checklist by name or path. It handles chaining automatically:
// if the run (or a condition item) triggers another checklist, that runs next.
func Run(nameOrPath string) error {
	return runChain(nameOrPath, uuid.New().String(), "", nil)
}

// runChain is the recursive engine for chained runs.
//   - chainID  : shared identifier for all runs in this chain
//   - triggeredBy: name of the checklist that triggered this one (empty for the first)
//   - visited  : names already in this chain (loop protection)
func runChain(nameOrPath, chainID, triggeredBy string, visited []string) error {
	cl, err := checklist.Load(nameOrPath)
	if err != nil {
		return fmt.Errorf("loading checklist: %w", err)
	}

	// Loop protection.
	for _, v := range visited {
		if v == cl.Name {
			return fmt.Errorf("chain loop detected: %q already ran in this chain", cl.Name)
		}
	}
	visited = append(visited, cl.Name)

	flat := cl.Flatten()
	completedBy := store.CompletedBy()

	// Resume or fresh start.
	state, err := session.Resume(cl.Name, nameOrPath, flat)
	if err != nil {
		return err
	}

	// Item-level automation callback (runs in background goroutine).
	onItemComplete := func(idx int, value string) {
		if idx >= len(flat) {
			return
		}
		item := flat[idx].Item
		if len(item.OnComplete) == 0 {
			return
		}
		payload := automation.ItemPayload{
			Event:         "item_completed",
			RunID:         state.RunID,
			ChecklistName: cl.Name,
			CompletedBy:   completedBy,
			Timestamp:     time.Now(),
			Item: automation.ItemData{
				ID:     item.ID,
				Label:  item.Label,
				Status: "checked",
				Value:  value,
			},
		}
		automation.RunSteps(item.OnComplete, payload) //nolint:errcheck
	}

	// Run the TUI.
	model := tui.New(cl, state, onItemComplete)
	p := tea.NewProgram(model)
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("running TUI: %w", err)
	}
	final, ok := result.(tui.Model)
	if !ok {
		return fmt.Errorf("unexpected model type after run")
	}

	// Handle outcome.
	switch {
	case !final.Done && !final.Saved:
		// Quit with no progress saved — nothing to do.

	case !final.Done && final.Saved:
		fmt.Println("  Progress saved.")

	case final.Done && final.Aborted:
		if err := store.AbortRun(final.State, completedBy, chainID, triggeredBy); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: could not save abort log: %v\n", err)
		}
		if final.TriggerNext != "" {
			return runChain(final.TriggerNext, chainID, cl.Name, visited)
		}

	case final.Done:
		if err := store.CompleteRun(final.State, completedBy, chainID, triggeredBy); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: could not save run log: %v\n", err)
		}
		runListAutomations(cl, final.State, completedBy)

		// List-level trigger (from trigger_checklist field or condition item).
		next := cl.TriggerChecklist
		if final.TriggerNext != "" {
			next = final.TriggerNext
		}
		if next != "" {
			return runChain(next, chainID, cl.Name, visited)
		}
	}

	return nil
}

// runListAutomations fires on_complete steps after a successful run.
func runListAutomations(cl *checklist.Checklist, state *store.RunState, completedBy string) {
	if len(cl.OnComplete) == 0 {
		return
	}
	now := time.Now()
	flat := cl.Flatten()
	items := make([]automation.ItemData, len(state.Items))
	for i, it := range state.Items {
		items[i] = automation.ItemData{
			ID:     it.ID,
			Status: string(it.Status),
			Value:  it.Value,
		}
		if i < len(flat) {
			items[i].Label = flat[i].Item.Label
		}
	}
	payload := automation.ChecklistPayload{
		Event:           "checklist_completed",
		RunID:           state.RunID,
		ChecklistName:   cl.Name,
		StartedAt:       state.StartedAt,
		CompletedAt:     now,
		DurationSeconds: now.Sub(state.StartedAt).Seconds(),
		CompletedBy:     completedBy,
		Items:           items,
	}
	errs := automation.RunSteps(cl.OnComplete, payload)
	for _, err := range errs {
		fmt.Fprintf(os.Stderr, "  automation warning: %v\n", err)
	}
}
