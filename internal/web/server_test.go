package web

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/benlutz/preflt/internal/checklist"
	"github.com/benlutz/preflt/internal/store"
)

func useTemp(t *testing.T) {
	t.Helper()
	t.Setenv("PREFLT_HOME", t.TempDir())
}

func mkServer(t *testing.T, items ...checklist.Item) *server {
	t.Helper()
	cl := &checklist.Checklist{
		Name:  "test-list",
		Items: items,
		Type:  checklist.TypeNormal,
	}
	flat := cl.Flatten()
	ids := make([]string, len(flat))
	for i, fi := range flat {
		ids[i] = fi.Item.ID
	}
	state := store.NewRunState(cl.Name, "./test.yaml", ids)
	_, cancel := context.WithCancel(context.Background())
	return &server{
		cl:          cl,
		items:       flat,
		state:       state,
		completedBy: "tester",
		chainID:     "test-chain",
		visited:     []string{cl.Name},
		chainStep:   1,
		cancel:      cancel,
	}
}

func apiState(t *testing.T, s *server) stateResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/state", nil)
	w := httptest.NewRecorder()
	s.handleState(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/state: expected 200, got %d", w.Code)
	}
	var resp stateResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding state response: %v", err)
	}
	return resp
}

func apiConfirm(t *testing.T, s *server, value string) (stateResponse, int) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"value": value})
	req := httptest.NewRequest(http.MethodPost, "/api/item/confirm", bytes.NewReader(body))
	w := httptest.NewRecorder()
	s.handleConfirm(w, req)
	var resp stateResponse
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	return resp, w.Code
}

func apiNA(t *testing.T, s *server) (stateResponse, int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/item/na", nil)
	w := httptest.NewRecorder()
	s.handleNA(w, req)
	var resp stateResponse
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	return resp, w.Code
}

// ── handleState ───────────────────────────────────────────────────────────────

func TestHandleState_basic(t *testing.T) {
	useTemp(t)
	s := mkServer(t,
		checklist.Item{ID: "a", Label: "Item A", Type: checklist.ItemDo},
		checklist.Item{ID: "b", Label: "Item B", Type: checklist.ItemDo},
	)
	resp := apiState(t, s)
	if resp.Name != "test-list" {
		t.Errorf("expected name 'test-list', got %q", resp.Name)
	}
	if len(resp.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(resp.Items))
	}
	if resp.Current != 0 {
		t.Errorf("expected current=0, got %d", resp.Current)
	}
	if resp.Done {
		t.Error("expected done=false")
	}
}

func TestHandleState_wrongMethod(t *testing.T) {
	useTemp(t)
	s := mkServer(t, checklist.Item{ID: "a", Label: "A", Type: checklist.ItemDo})
	req := httptest.NewRequest(http.MethodPost, "/api/state", nil)
	w := httptest.NewRecorder()
	s.handleState(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// ── handleConfirm ─────────────────────────────────────────────────────────────

func TestHandleConfirm_advancesCursor(t *testing.T) {
	useTemp(t)
	s := mkServer(t,
		checklist.Item{ID: "a", Label: "A", Type: checklist.ItemDo},
		checklist.Item{ID: "b", Label: "B", Type: checklist.ItemDo},
	)
	resp, code := apiConfirm(t, s, "")
	if code != http.StatusOK {
		t.Errorf("expected 200, got %d", code)
	}
	if resp.Current != 1 {
		t.Errorf("expected current=1 after confirming item 0, got %d", resp.Current)
	}
	if resp.Done {
		t.Error("expected done=false after first confirm")
	}
}

func TestHandleConfirm_lastItemSetsDone(t *testing.T) {
	useTemp(t)
	s := mkServer(t, checklist.Item{ID: "a", Label: "A", Type: checklist.ItemDo})
	resp, _ := apiConfirm(t, s, "")
	if !resp.Done {
		t.Error("expected done=true after confirming only item")
	}
	if resp.Current != -1 {
		t.Errorf("expected current=-1 when done, got %d", resp.Current)
	}
}

func TestHandleConfirm_capturesValue(t *testing.T) {
	useTemp(t)
	s := mkServer(t, checklist.Item{ID: "a", Label: "A", Type: checklist.ItemCheck})
	apiConfirm(t, s, "my answer") //nolint:errcheck
	if s.state.Items[0].Value != "my answer" {
		t.Errorf("expected value 'my answer', got %q", s.state.Items[0].Value)
	}
}

func TestHandleConfirm_wrongMethod(t *testing.T) {
	useTemp(t)
	s := mkServer(t, checklist.Item{ID: "a", Label: "A", Type: checklist.ItemDo})
	req := httptest.NewRequest(http.MethodGet, "/api/item/confirm", nil)
	w := httptest.NewRecorder()
	s.handleConfirm(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleConfirm_idempotentWhenDone(t *testing.T) {
	useTemp(t)
	s := mkServer(t, checklist.Item{ID: "a", Label: "A", Type: checklist.ItemDo})
	apiConfirm(t, s, "") //nolint:errcheck
	// Confirm again after done — should return current state without panicking.
	resp, code := apiConfirm(t, s, "")
	if code != http.StatusOK {
		t.Errorf("expected 200, got %d", code)
	}
	if !resp.Done {
		t.Error("expected done=true")
	}
}

// ── handleNA ──────────────────────────────────────────────────────────────────

func TestHandleNA_allowed(t *testing.T) {
	useTemp(t)
	s := mkServer(t, checklist.Item{ID: "a", Label: "A", Type: checklist.ItemDo, NAAllowed: true})
	_, code := apiNA(t, s)
	if code != http.StatusOK {
		t.Errorf("expected 200, got %d", code)
	}
	if s.state.Items[0].Status != store.StatusNA {
		t.Errorf("expected status NA, got %q", s.state.Items[0].Status)
	}
}

func TestHandleNA_notAllowed(t *testing.T) {
	useTemp(t)
	s := mkServer(t, checklist.Item{ID: "a", Label: "A", Type: checklist.ItemDo, NAAllowed: false})
	_, code := apiNA(t, s)
	if code != http.StatusBadRequest {
		t.Errorf("expected 400 for item with na_allowed=false, got %d", code)
	}
	if s.state.Items[0].Status != store.StatusPending {
		t.Errorf("expected item to remain pending, got %q", s.state.Items[0].Status)
	}
}

func TestHandleNA_wrongMethod(t *testing.T) {
	useTemp(t)
	s := mkServer(t, checklist.Item{ID: "a", Label: "A", Type: checklist.ItemDo, NAAllowed: true})
	req := httptest.NewRequest(http.MethodGet, "/api/item/na", nil)
	w := httptest.NewRecorder()
	s.handleNA(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestHandleNA_lastItemSetsDone(t *testing.T) {
	useTemp(t)
	s := mkServer(t, checklist.Item{ID: "a", Label: "A", Type: checklist.ItemDo, NAAllowed: true})
	resp, _ := apiNA(t, s)
	if !resp.Done {
		t.Error("expected done=true after N/A on only item")
	}
}

// ── condition branch ──────────────────────────────────────────────────────────

func TestHandleConfirm_condition_yes_confirms(t *testing.T) {
	useTemp(t)
	item := checklist.Item{
		ID:    "q",
		Label: "Ready?",
		Type:  checklist.ItemDo,
		Condition: &checklist.Condition{
			IfYes: &checklist.ConditionBranch{},
			IfNo:  &checklist.ConditionBranch{Skip: true},
		},
	}
	s := mkServer(t, item, checklist.Item{ID: "next", Label: "Next", Type: checklist.ItemDo})
	resp, _ := apiConfirm(t, s, "yes")
	if resp.Current != 1 {
		t.Errorf("expected current=1 after yes branch, got %d", resp.Current)
	}
	if s.state.Items[0].Value != "yes" {
		t.Errorf("expected value 'yes', got %q", s.state.Items[0].Value)
	}
}

func TestHandleConfirm_condition_no_skip(t *testing.T) {
	useTemp(t)
	item := checklist.Item{
		ID:    "q",
		Label: "Ready?",
		Type:  checklist.ItemDo,
		Condition: &checklist.Condition{
			IfYes: &checklist.ConditionBranch{},
			IfNo:  &checklist.ConditionBranch{Skip: true},
		},
	}
	s := mkServer(t, item, checklist.Item{ID: "next", Label: "Next", Type: checklist.ItemDo})
	apiConfirm(t, s, "no") //nolint:errcheck
	if s.state.Items[0].Status != store.StatusNA {
		t.Errorf("expected status NA for skip branch, got %q", s.state.Items[0].Status)
	}
}

// ── branchFor ─────────────────────────────────────────────────────────────────

func TestBranchFor_yes(t *testing.T) {
	yes := &checklist.ConditionBranch{Skip: true}
	no := &checklist.ConditionBranch{Abort: true}
	cond := &checklist.Condition{IfYes: yes, IfNo: no}
	if branchFor(cond, "yes") != yes {
		t.Error("expected IfYes branch for 'yes'")
	}
	if branchFor(cond, "YES") != yes {
		t.Error("expected IfYes branch for 'YES' (case-insensitive)")
	}
}

func TestBranchFor_no(t *testing.T) {
	no := &checklist.ConditionBranch{Abort: true}
	cond := &checklist.Condition{IfNo: no}
	if branchFor(cond, "no") != no {
		t.Error("expected IfNo branch for 'no'")
	}
}

func TestBranchFor_nilCondition(t *testing.T) {
	if branchFor(nil, "yes") != nil {
		t.Error("expected nil for nil condition")
	}
}

// ── currentIndex ─────────────────────────────────────────────────────────────

func TestCurrentIndex_allPending(t *testing.T) {
	useTemp(t)
	s := mkServer(t,
		checklist.Item{ID: "a", Label: "A", Type: checklist.ItemDo},
		checklist.Item{ID: "b", Label: "B", Type: checklist.ItemDo},
	)
	if s.currentIndex() != 0 {
		t.Errorf("expected 0, got %d", s.currentIndex())
	}
}

func TestCurrentIndex_firstDone(t *testing.T) {
	useTemp(t)
	s := mkServer(t,
		checklist.Item{ID: "a", Label: "A", Type: checklist.ItemDo},
		checklist.Item{ID: "b", Label: "B", Type: checklist.ItemDo},
	)
	s.state.Items[0].Status = store.StatusChecked
	if s.currentIndex() != 1 {
		t.Errorf("expected 1, got %d", s.currentIndex())
	}
}

func TestCurrentIndex_allDone(t *testing.T) {
	useTemp(t)
	s := mkServer(t,
		checklist.Item{ID: "a", Label: "A", Type: checklist.ItemDo},
		checklist.Item{ID: "b", Label: "B", Type: checklist.ItemDo},
	)
	s.state.Items[0].Status = store.StatusChecked
	s.state.Items[1].Status = store.StatusNA
	if s.currentIndex() != -1 {
		t.Errorf("expected -1 when all done, got %d", s.currentIndex())
	}
}
