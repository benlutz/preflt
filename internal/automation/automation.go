package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/benlutz/preflt/internal/checklist"
)

const (
	shellTimeout   = 30 * time.Second
	webhookTimeout = 10 * time.Second
)

// ItemPayload is the webhook body sent when a single item is confirmed.
type ItemPayload struct {
	Event         string    `json:"event"` // "item_completed"
	RunID         string    `json:"run_id"`
	ChecklistName string    `json:"checklist_name"`
	CompletedBy   string    `json:"completed_by"`
	Timestamp     time.Time `json:"timestamp"`
	Item          ItemData  `json:"item"`
}

// ChecklistPayload is the webhook body sent when a full checklist completes.
type ChecklistPayload struct {
	Event           string     `json:"event"` // "checklist_completed"
	RunID           string     `json:"run_id"`
	ChecklistName   string     `json:"checklist_name"`
	StartedAt       time.Time  `json:"started_at"`
	CompletedAt     time.Time  `json:"completed_at"`
	DurationSeconds float64    `json:"duration_seconds"`
	CompletedBy     string     `json:"completed_by"`
	Items           []ItemData `json:"items"`
}

// ItemData is the per-item summary included in both payload types.
type ItemData struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status string `json:"status"`
	Value  string `json:"value,omitempty"`
}

// RunSteps executes each step in order. Errors are collected and returned but
// never stop execution — a failed automation never blocks checklist progress.
func RunSteps(steps []checklist.AutomationStep, payload any) []error {
	var errs []error
	for _, step := range steps {
		if step.Shell != "" {
			if err := runShell(step.Shell); err != nil {
				errs = append(errs, fmt.Errorf("shell %q: %w", step.Shell, err))
			}
		}
		if step.Webhook != "" {
			if err := runWebhook(step.Webhook, payload); err != nil {
				errs = append(errs, fmt.Errorf("webhook %q: %w", step.Webhook, err))
			}
		}
	}
	return errs
}

func runShell(cmd string) error {
	ctx, cancel := context.WithTimeout(context.Background(), shellTimeout)
	defer cancel()

	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	var stderr bytes.Buffer
	c.Stderr = &stderr

	if err := c.Run(); err != nil {
		if stderr.Len() > 0 {
			return fmt.Errorf("%w: %s", err, stderr.String())
		}
		return err
	}
	return nil
}

func runWebhook(url string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshalling payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), webhookTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "preflt")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}
