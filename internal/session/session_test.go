package session_test

import (
	"os"
	"testing"

	"github.com/benlutz/preflt/internal/checklist"
	"github.com/benlutz/preflt/internal/session"
	"github.com/benlutz/preflt/internal/store"
)

func useTemp(t *testing.T) {
	t.Helper()
	t.Setenv("PREFLT_HOME", t.TempDir())
}

func flat(ids ...string) []checklist.FlatItem {
	items := make([]checklist.FlatItem, len(ids))
	for i, id := range ids {
		items[i] = checklist.FlatItem{
			Item:      checklist.Item{ID: id, Label: "item " + id},
			GlobalIdx: i,
		}
	}
	return items
}

func injectStdin(t *testing.T, input string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = old
		r.Close()
	})
	if _, err := w.WriteString(input); err != nil {
		t.Fatal(err)
	}
	w.Close()
}

func TestResume_freshStart(t *testing.T) {
	useTemp(t)

	state, err := session.Resume("mylist", "./mylist.yaml", flat("a", "b"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state == nil {
		t.Fatal("expected a state, got nil")
	}
	if state.RunID == "" {
		t.Error("expected non-empty RunID")
	}
	if len(state.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(state.Items))
	}
}

func TestResume_staleStateDiscarded(t *testing.T) {
	useTemp(t)

	existing := store.NewRunState("mylist", "./mylist.yaml", []string{"a", "b"})
	if err := store.SaveState(existing); err != nil {
		t.Fatal(err)
	}

	// Different item IDs — stale session should be discarded without prompting.
	state, err := session.Resume("mylist", "./mylist.yaml", flat("x", "y", "z"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.RunID == existing.RunID {
		t.Error("expected a new RunID, got the stale one")
	}
	if len(state.Items) != 3 {
		t.Errorf("expected 3 items for fresh state, got %d", len(state.Items))
	}
}

func TestResume_acceptPrompt(t *testing.T) {
	useTemp(t)

	existing := store.NewRunState("mylist", "./mylist.yaml", []string{"a", "b"})
	if err := store.SaveState(existing); err != nil {
		t.Fatal(err)
	}

	injectStdin(t, "y\n")

	state, err := session.Resume("mylist", "./mylist.yaml", flat("a", "b"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.RunID != existing.RunID {
		t.Errorf("expected to resume run %q, got %q", existing.RunID, state.RunID)
	}
}

func TestResume_declinePrompt(t *testing.T) {
	useTemp(t)

	existing := store.NewRunState("mylist", "./mylist.yaml", []string{"a", "b"})
	if err := store.SaveState(existing); err != nil {
		t.Fatal(err)
	}

	injectStdin(t, "n\n")

	state, err := session.Resume("mylist", "./mylist.yaml", flat("a", "b"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.RunID == existing.RunID {
		t.Error("expected a new RunID after declining resume")
	}
}
