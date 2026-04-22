package store

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ItemStatus describes the resolution of a single checklist item.
type ItemStatus string

const (
	StatusPending ItemStatus = "pending"
	StatusChecked ItemStatus = "checked"
	StatusNA      ItemStatus = "na"
)

// ItemState holds the runtime status of a single item.
type ItemState struct {
	ID        string     `json:"id"`
	Status    ItemStatus `json:"status"`
	Value     string     `json:"value,omitempty"`
	CheckedAt *time.Time `json:"checked_at,omitempty"`
}

// RunState is the in-progress state persisted while a checklist is running.
type RunState struct {
	RunID         string      `json:"run_id"`
	ChecklistName string      `json:"checklist_name"`
	ChecklistPath string      `json:"checklist_path"`
	StartedAt     time.Time   `json:"started_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
	CurrentIndex  int         `json:"current_index"`
	Items         []ItemState `json:"items"`
}

// RunLog is the completed record written at the end of a run.
type RunLog struct {
	RunID         string      `json:"run_id"`
	ChecklistName string      `json:"checklist_name"`
	StartedAt     time.Time   `json:"started_at"`
	CompletedAt   time.Time   `json:"completed_at"`
	CompletedBy   string      `json:"completed_by"`
	Status        string      `json:"status"` // "completed" | "aborted"
	Items         []ItemState `json:"items"`
	ChainID       string      `json:"chain_id,omitempty"`       // shared across a run chain
	TriggeredBy   string      `json:"triggered_by,omitempty"`   // checklist that triggered this one
}

// prefltHome returns the base data directory for preflt.
// The PREFLT_HOME environment variable overrides the default ~/.preflt.
func prefltHome() (string, error) {
	if dir := os.Getenv("PREFLT_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home directory: %w", err)
	}
	return filepath.Join(home, ".preflt"), nil
}

// runsDir returns the path to the runs directory.
func runsDir() (string, error) {
	home, err := prefltHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "runs"), nil
}

// NewRunState creates a fresh RunState for the given checklist.
func NewRunState(name, path string, itemIDs []string) *RunState {
	items := make([]ItemState, len(itemIDs))
	for i, id := range itemIDs {
		items[i] = ItemState{
			ID:     id,
			Status: StatusPending,
		}
	}
	now := time.Now()
	return &RunState{
		RunID:         uuid.New().String(),
		ChecklistName: name,
		ChecklistPath: path,
		StartedAt:     now,
		UpdatedAt:     now,
		CurrentIndex:  0,
		Items:         items,
	}
}

// runDir returns the directory for a specific run.
func runDir(runID string) (string, error) {
	base, err := runsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, runID), nil
}

// SaveState persists the current RunState to disk.
func SaveState(state *RunState) error {
	state.UpdatedAt = time.Now()

	dir, err := runDir(state.RunID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating run directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling state: %w", err)
	}

	return os.WriteFile(filepath.Join(dir, "state.json"), data, 0644)
}

// LoadLatestState finds the most recently updated in-progress run for the
// given checklist name. Returns nil, nil when no in-progress run exists.
func LoadLatestState(checklistName string) (*RunState, error) {
	base, err := runsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading runs directory: %w", err)
	}

	var latest *RunState
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		statePath := filepath.Join(base, e.Name(), "state.json")
		data, err := os.ReadFile(statePath)
		if err != nil {
			continue // no state.json → completed run
		}
		var rs RunState
		if err := json.Unmarshal(data, &rs); err != nil {
			continue
		}
		if rs.ChecklistName != checklistName {
			continue
		}
		if latest == nil || rs.UpdatedAt.After(latest.UpdatedAt) {
			tmp := rs
			latest = &tmp
		}
	}

	return latest, nil
}

// DiscardState removes all trace of an in-progress run. Used when the user
// explicitly chooses not to save their progress on quit.
func DiscardState(state *RunState) error {
	dir, err := runDir(state.RunID)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

// AbortRun writes a run log with status "aborted" and removes the state file.
func AbortRun(state *RunState, by, chainID, triggeredBy string) error {
	log := RunLog{
		RunID:         state.RunID,
		ChecklistName: state.ChecklistName,
		StartedAt:     state.StartedAt,
		CompletedAt:   time.Now(),
		CompletedBy:   by,
		Status:        "aborted",
		Items:         state.Items,
		ChainID:       chainID,
		TriggeredBy:   triggeredBy,
	}
	dir, err := runDir(state.RunID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "run.json"), data, 0644); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(dir, "state.json"))
	return nil
}

// CompleteRun writes the run log and removes the in-progress state file.
func CompleteRun(state *RunState, by, chainID, triggeredBy string) error {
	log := RunLog{
		RunID:         state.RunID,
		ChecklistName: state.ChecklistName,
		StartedAt:     state.StartedAt,
		CompletedAt:   time.Now(),
		CompletedBy:   by,
		Status:        "completed",
		Items:         state.Items,
		ChainID:       chainID,
		TriggeredBy:   triggeredBy,
	}

	dir, err := runDir(state.RunID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating run directory: %w", err)
	}

	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling run log: %w", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "run.json"), data, 0644); err != nil {
		return fmt.Errorf("writing run log: %w", err)
	}

	// Remove the in-progress state file.
	_ = os.Remove(filepath.Join(dir, "state.json"))
	return nil
}

// LoadHistory returns completed run logs for the given checklist name, sorted
// by CompletedAt descending, up to limit entries.
func LoadHistory(name string, limit int) ([]*RunLog, error) {
	base, err := runsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading runs directory: %w", err)
	}

	var logs []*RunLog
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		logPath := filepath.Join(base, e.Name(), "run.json")
		data, err := os.ReadFile(logPath)
		if err != nil {
			continue
		}
		var rl RunLog
		if err := json.Unmarshal(data, &rl); err != nil {
			continue
		}
		if rl.ChecklistName != name {
			continue
		}
		tmp := rl
		logs = append(logs, &tmp)
	}

	sort.Slice(logs, func(i, j int) bool {
		return logs[i].CompletedAt.After(logs[j].CompletedAt)
	})

	if limit > 0 && len(logs) > limit {
		logs = logs[:limit]
	}

	return logs, nil
}

// CompletedBy returns a human-readable identifier for the current user.
// It tries git config user.name, then the system hostname, then "unknown".
func CompletedBy() string {
	out, err := exec.Command("git", "config", "user.name").Output()
	if err == nil {
		name := strings.TrimSpace(string(out))
		if name != "" {
			return name
		}
	}

	host, err := os.Hostname()
	if err == nil && host != "" {
		return host
	}

	return "unknown"
}
