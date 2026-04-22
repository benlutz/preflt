package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/benlutz/preflt/internal/automation"
	"github.com/benlutz/preflt/internal/checklist"
	"github.com/benlutz/preflt/internal/session"
	"github.com/benlutz/preflt/internal/store"
	"github.com/google/uuid"
)

//go:embed static
var staticFiles embed.FS

// server holds the runtime state for a web-based checklist run.
// It supports chaining — when one checklist triggers another, the server
// swaps its active checklist in-place and keeps serving.
type server struct {
	cl          *checklist.Checklist
	items       []checklist.FlatItem
	state       *store.RunState
	completedBy string

	// chain state
	chainID     string
	visited     []string // names already run — loop protection
	triggerNext string   // set by condition branch or list-level field
	triggeredBy string   // name of the checklist that triggered this one
	chainStep   int      // 1-indexed position in the chain

	mu          sync.Mutex
	srv         *http.Server
	done        bool       // true when the current checklist is complete (no next)
	completedAt time.Time
	cancel      context.CancelFunc
}

// stateResponse is the JSON shape returned by all API endpoints.
type stateResponse struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Items       []itemView `json:"items"`
	Current     int        `json:"current"`   // first pending item index, -1 when all done
	Done        bool       `json:"done"`
	StartedAt   time.Time  `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	TriggeredBy string     `json:"triggeredBy,omitempty"` // previous checklist in chain
	ChainStep   int        `json:"chainStep"`             // 1-indexed
}

type itemView struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Response  string `json:"response"`
	Note      string `json:"note"`
	Type      string `json:"type"`
	NAAllowed bool   `json:"naAllowed"`
	Phase     string `json:"phase"`
	Status    string `json:"status"`
	Value     string `json:"value"`
}

// Serve loads a checklist by name or path, handles the resume prompt in the
// terminal, starts the HTTP server, opens the browser, and blocks until done.
func Serve(nameOrPath, host string, port int) error {
	cl, err := checklist.Load(nameOrPath)
	if err != nil {
		return fmt.Errorf("loading checklist: %w", err)
	}

	flat := cl.Flatten()
	completedBy := store.CompletedBy()

	// Resume or fresh start.
	state, err := session.Resume(cl.Name, nameOrPath, flat)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &server{
		cl:          cl,
		items:       flat,
		state:       state,
		completedBy: completedBy,
		chainID:     uuid.New().String(),
		visited:     []string{cl.Name},
		chainStep:   1,
		cancel:      cancel,
	}

	mux := http.NewServeMux()

	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return err
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/item/confirm", s.handleConfirm)
	mux.HandleFunc("/api/item/na", s.handleNA)

	addr := fmt.Sprintf("%s:%d", host, port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("could not bind to %s: %w\n  (Is another preflt web session already running?)", addr, err)
	}

	s.srv = &http.Server{Addr: addr, Handler: mux}

	displayHost := host
	if host == "0.0.0.0" {
		displayHost = "localhost"
	}
	url := fmt.Sprintf("http://%s:%d", displayHost, port)
	fmt.Printf("\n  preflt web — %s\n  %s\n\n  Press Ctrl+C to stop.\n\n", cl.Name, url)
	openBrowser(url)

	go func() {
		<-ctx.Done()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutCancel()
		s.srv.Shutdown(shutCtx) //nolint:errcheck
	}()

	if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// ── API handlers ─────────────────────────────────────────────────────────────

func (s *server) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, s.buildResponse())
}

func (s *server) handleConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cur := s.currentIndex()
	if cur < 0 || s.done {
		writeJSON(w, s.buildResponse())
		return
	}

	now := time.Now()
	item := s.items[cur].Item

	if item.Condition != nil {
		// Evaluate the condition branch — mirrors TUI applyBranch().
		branch := branchFor(item.Condition, body.Value)

		if branch != nil && branch.Skip {
			// Skip = mark as N/A and continue.
			s.state.Items[cur].Status = store.StatusNA
			s.state.Items[cur].CheckedAt = &now
		} else {
			// Confirm with the yes/no answer and fire item automation.
			s.state.Items[cur].Status = store.StatusChecked
			s.state.Items[cur].CheckedAt = &now
			s.state.Items[cur].Value = body.Value
			go s.runItemAutomation(cur, body.Value)

			if branch != nil && branch.TriggerChecklist != "" {
				s.triggerNext = branch.TriggerChecklist
				if branch.Abort {
					// Abort current checklist immediately and load the next one.
					s.abortAndTrigger()
					writeJSON(w, s.buildResponse())
					return
				}
			}
		}
	} else {
		// Regular do/check item.
		s.state.Items[cur].Status = store.StatusChecked
		s.state.Items[cur].CheckedAt = &now
		s.state.Items[cur].Value = body.Value
		go s.runItemAutomation(cur, body.Value)
	}

	if s.currentIndex() < 0 {
		s.complete()
	} else {
		store.SaveState(s.state) //nolint:errcheck
	}

	writeJSON(w, s.buildResponse())
}

func (s *server) handleNA(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	cur := s.currentIndex()
	if cur < 0 || s.done {
		writeJSON(w, s.buildResponse())
		return
	}

	now := time.Now()
	s.state.Items[cur].Status = store.StatusNA
	s.state.Items[cur].CheckedAt = &now

	if s.currentIndex() < 0 {
		s.complete()
	} else {
		store.SaveState(s.state) //nolint:errcheck
	}

	writeJSON(w, s.buildResponse())
}

// ── Chain logic ───────────────────────────────────────────────────────────────

// complete finalises the current checklist and, if something should run next,
// loads it. Otherwise it schedules server shutdown.
// Must be called with s.mu held.
func (s *server) complete() {
	s.completedAt = time.Now()
	store.CompleteRun(s.state, s.completedBy, s.chainID, s.triggeredBy) //nolint:errcheck
	go s.runListAutomations()

	next := s.triggerNext
	if next == "" {
		next = s.cl.TriggerChecklist
	}

	if next != "" && s.loadNext(next) {
		// Server continues with the new checklist — don't shutdown.
		return
	}

	// Nothing left to run.
	s.done = true
	go func() {
		time.Sleep(10 * time.Second)
		s.cancel()
	}()
}

// abortAndTrigger writes an abort log for the current checklist, then loads
// the next one. If no next checklist is set it marks the server as done.
// Must be called with s.mu held.
func (s *server) abortAndTrigger() {
	s.completedAt = time.Now()
	store.AbortRun(s.state, s.completedBy, s.chainID, s.triggeredBy) //nolint:errcheck

	if s.triggerNext != "" && s.loadNext(s.triggerNext) {
		return
	}

	s.done = true
	go func() {
		time.Sleep(10 * time.Second)
		s.cancel()
	}()
}

// loadNext loads the next checklist in the chain and swaps the server state.
// Returns true if loading succeeded, false on error or loop detection.
// Must be called with s.mu held.
func (s *server) loadNext(nameOrPath string) bool {
	// Check visited list first (fast path for name-based triggers).
	for _, v := range s.visited {
		if v == nameOrPath {
			fmt.Fprintf(os.Stderr, "  web chain: loop detected — %q already ran in this chain\n", nameOrPath)
			return false
		}
	}

	cl, err := checklist.Load(nameOrPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  web chain: could not load %q: %v\n", nameOrPath, err)
		return false
	}

	// Also check by resolved name.
	for _, v := range s.visited {
		if v == cl.Name {
			fmt.Fprintf(os.Stderr, "  web chain: loop detected — %q already ran in this chain\n", cl.Name)
			return false
		}
	}

	flat := cl.Flatten()
	itemIDs := make([]string, len(flat))
	for i, fi := range flat {
		itemIDs[i] = fi.Item.ID
	}

	prevName := s.cl.Name
	s.cl = cl
	s.items = flat
	s.state = store.NewRunState(cl.Name, nameOrPath, itemIDs)
	s.done = false
	s.triggeredBy = prevName
	s.triggerNext = ""
	s.chainStep++
	s.visited = append(s.visited, cl.Name)

	store.SaveState(s.state) //nolint:errcheck
	return true
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// currentIndex returns the index of the first pending item, or -1 if none.
// Must be called with s.mu held.
func (s *server) currentIndex() int {
	for i, it := range s.state.Items {
		if it.Status == store.StatusPending {
			return i
		}
	}
	return -1
}

// branchFor returns the ConditionBranch that matches the given answer ("yes"/"no").
func branchFor(cond *checklist.Condition, answer string) *checklist.ConditionBranch {
	if cond == nil {
		return nil
	}
	if strings.ToLower(answer) == "yes" {
		return cond.IfYes
	}
	return cond.IfNo
}

func (s *server) buildResponse() stateResponse {
	views := make([]itemView, len(s.items))
	for i, fi := range s.items {
		st := s.state.Items[i]
		itemType := string(fi.Item.Type)
		if fi.Item.Condition != nil {
			itemType = "condition"
		}
		views[i] = itemView{
			ID:        fi.Item.ID,
			Label:     fi.Item.Label,
			Response:  fi.Item.Response,
			Note:      fi.Item.Note,
			Type:      itemType,
			NAAllowed: fi.Item.NAAllowed,
			Phase:     fi.PhaseName,
			Status:    string(st.Status),
			Value:     st.Value,
		}
	}
	resp := stateResponse{
		Name:        s.cl.Name,
		Description: s.cl.Description,
		Items:       views,
		Current:     s.currentIndex(),
		Done:        s.done,
		StartedAt:   s.state.StartedAt,
		TriggeredBy: s.triggeredBy,
		ChainStep:   s.chainStep,
	}
	if s.done {
		resp.CompletedAt = &s.completedAt
	}
	return resp
}

func (s *server) runItemAutomation(idx int, value string) {
	item := s.items[idx].Item
	if len(item.OnComplete) == 0 {
		return
	}
	payload := automation.ItemPayload{
		Event:         "item_completed",
		RunID:         s.state.RunID,
		ChecklistName: s.cl.Name,
		CompletedBy:   s.completedBy,
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

func (s *server) runListAutomations() {
	if len(s.cl.OnComplete) == 0 {
		return
	}
	items := make([]automation.ItemData, len(s.state.Items))
	for i, it := range s.state.Items {
		items[i] = automation.ItemData{
			ID:     it.ID,
			Status: string(it.Status),
			Value:  it.Value,
		}
		if i < len(s.items) {
			items[i].Label = s.items[i].Item.Label
		}
	}
	payload := automation.ChecklistPayload{
		Event:           "checklist_completed",
		RunID:           s.state.RunID,
		ChecklistName:   s.cl.Name,
		StartedAt:       s.state.StartedAt,
		CompletedAt:     s.completedAt,
		DurationSeconds: s.completedAt.Sub(s.state.StartedAt).Seconds(),
		CompletedBy:     s.completedBy,
		Items:           items,
	}
	automation.RunSteps(s.cl.OnComplete, payload) //nolint:errcheck
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{url}
	case "windows":
		cmd, args = "cmd", []string{"/c", "start", url}
	default:
		cmd, args = "xdg-open", []string{url}
	}
	exec.Command(cmd, args...).Start() //nolint:errcheck
}
