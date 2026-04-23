package automation_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/benlutz/preflt/internal/automation"
	"github.com/benlutz/preflt/internal/checklist"
)

func TestRunSteps_empty(t *testing.T) {
	errs := automation.RunSteps(nil, nil)
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty steps, got: %v", errs)
	}
}

func TestRunSteps_shell_success(t *testing.T) {
	steps := []checklist.AutomationStep{{Shell: "true"}}
	errs := automation.RunSteps(steps, nil)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestRunSteps_shell_failure(t *testing.T) {
	steps := []checklist.AutomationStep{{Shell: "false"}}
	errs := automation.RunSteps(steps, nil)
	if len(errs) != 1 {
		t.Errorf("expected 1 error, got %d", len(errs))
	}
}

func TestRunSteps_shell_capturesStderr(t *testing.T) {
	steps := []checklist.AutomationStep{{Shell: "echo oops >&2; exit 1"}}
	errs := automation.RunSteps(steps, nil)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestRunSteps_webhook_success(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received) //nolint:errcheck
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	payload := map[string]string{"event": "test"}
	steps := []checklist.AutomationStep{{Webhook: srv.URL}}
	errs := automation.RunSteps(steps, payload)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
	if received["event"] != "test" {
		t.Errorf("webhook payload not received correctly: %v", received)
	}
}

func TestRunSteps_webhook_serverError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	steps := []checklist.AutomationStep{{Webhook: srv.URL}}
	errs := automation.RunSteps(steps, nil)
	if len(errs) != 1 {
		t.Errorf("expected 1 error for 500 response, got %d", len(errs))
	}
}

func TestRunSteps_webhook_setsContentType(t *testing.T) {
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	steps := []checklist.AutomationStep{{Webhook: srv.URL}}
	automation.RunSteps(steps, nil) //nolint:errcheck
	if gotCT != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", gotCT)
	}
}

func TestRunSteps_shellAndWebhook_bothRun(t *testing.T) {
	webhookCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	steps := []checklist.AutomationStep{{Shell: "true", Webhook: srv.URL}}
	errs := automation.RunSteps(steps, nil)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
	if !webhookCalled {
		t.Error("expected webhook to be called")
	}
}

func TestRunSteps_shellFailureDoesNotBlockWebhook(t *testing.T) {
	webhookCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		webhookCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	steps := []checklist.AutomationStep{{Shell: "false", Webhook: srv.URL}}
	errs := automation.RunSteps(steps, nil)
	if len(errs) != 1 {
		t.Errorf("expected 1 error (shell failure), got %d", len(errs))
	}
	if !webhookCalled {
		t.Error("expected webhook to still be called after shell failure")
	}
}

func TestRunSteps_multipleStepsCollectAllErrors(t *testing.T) {
	steps := []checklist.AutomationStep{
		{Shell: "false"},
		{Shell: "false"},
	}
	errs := automation.RunSteps(steps, nil)
	if len(errs) != 2 {
		t.Errorf("expected 2 errors, got %d", len(errs))
	}
}
