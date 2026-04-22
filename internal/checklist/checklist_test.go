package checklist_test

import (
	"os"
	"testing"

	. "github.com/benlutz/preflt/internal/checklist"
)

// writeYAML writes content to a temp file and returns its path.
func writeYAML(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

// ── Flatten ──────────────────────────────────────────────────────────────────

func TestFlatten_flat(t *testing.T) {
	cl := &Checklist{
		Name:  "test",
		Items: []Item{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}},
	}
	flat := cl.Flatten()
	if len(flat) != 2 {
		t.Fatalf("expected 2 items, got %d", len(flat))
	}
	if flat[0].PhaseName != "" || flat[1].PhaseName != "" {
		t.Error("expected no phase names for flat checklist")
	}
	if flat[0].GlobalIdx != 0 || flat[1].GlobalIdx != 1 {
		t.Errorf("wrong global indices: %d, %d", flat[0].GlobalIdx, flat[1].GlobalIdx)
	}
}

func TestFlatten_phased(t *testing.T) {
	cl := &Checklist{
		Name: "test",
		Phases: []Phase{
			{Name: "P1", Items: []Item{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}}},
			{Name: "P2", Items: []Item{{ID: "c", Label: "C"}}},
		},
	}
	flat := cl.Flatten()
	if len(flat) != 3 {
		t.Fatalf("expected 3 items, got %d", len(flat))
	}
	if flat[0].PhaseName != "P1" || flat[1].PhaseName != "P1" {
		t.Errorf("wrong phase for items 0/1: %q, %q", flat[0].PhaseName, flat[1].PhaseName)
	}
	if flat[2].PhaseName != "P2" {
		t.Errorf("wrong phase for item 2: %q", flat[2].PhaseName)
	}
	if flat[2].GlobalIdx != 2 {
		t.Errorf("expected global index 2, got %d", flat[2].GlobalIdx)
	}
}

func TestFlatten_empty(t *testing.T) {
	cl := &Checklist{Name: "empty"}
	flat := cl.Flatten()
	if flat != nil {
		t.Errorf("expected nil for empty checklist, got %v", flat)
	}
}

// ── Validate ─────────────────────────────────────────────────────────────────

func TestValidate_valid(t *testing.T) {
	cl := &Checklist{
		Name:  "test",
		Items: []Item{{ID: "i1", Label: "Do something"}},
	}
	if err := cl.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidate_noName(t *testing.T) {
	cl := &Checklist{
		Items: []Item{{ID: "i1", Label: "Do something"}},
	}
	if err := cl.Validate(); err == nil {
		t.Error("expected error for missing name")
	}
}

func TestValidate_noItems(t *testing.T) {
	cl := &Checklist{Name: "empty"}
	if err := cl.Validate(); err == nil {
		t.Error("expected error for checklist with no items")
	}
}

func TestValidate_emptyLabel(t *testing.T) {
	cl := &Checklist{
		Name:  "test",
		Items: []Item{{ID: "i1", Label: ""}},
	}
	if err := cl.Validate(); err == nil {
		t.Error("expected error for item with empty label")
	}
}

func TestValidate_duplicateIDs(t *testing.T) {
	cl := &Checklist{
		Name: "test",
		Items: []Item{
			{ID: "dup", Label: "First"},
			{ID: "dup", Label: "Second"},
		},
	}
	if err := cl.Validate(); err == nil {
		t.Error("expected error for duplicate item IDs")
	}
}

func TestValidate_duplicateIDsAcrossPhases(t *testing.T) {
	cl := &Checklist{
		Name: "test",
		Phases: []Phase{
			{Name: "A", Items: []Item{{ID: "dup", Label: "First"}}},
			{Name: "B", Items: []Item{{ID: "dup", Label: "Second"}}},
		},
	}
	if err := cl.Validate(); err == nil {
		t.Error("expected error for duplicate IDs across phases")
	}
}

// ── Load ─────────────────────────────────────────────────────────────────────

func TestLoad_valid(t *testing.T) {
	path := writeYAML(t, `
name: deploy
description: Deploy checklist
type: normal
items:
  - id: tests
    label: Run tests
    type: do
`)
	cl, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cl.Name != "deploy" {
		t.Errorf("expected name 'deploy', got %q", cl.Name)
	}
	if cl.Type != TypeNormal {
		t.Errorf("expected type 'normal', got %q", cl.Type)
	}
	if len(cl.Items) != 1 || cl.Items[0].ID != "tests" {
		t.Errorf("unexpected items: %+v", cl.Items)
	}
}

func TestLoad_appliesDefaults(t *testing.T) {
	path := writeYAML(t, `
name: minimal
items:
  - label: Do something
  - label: Do another thing
`)
	cl, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cl.Type != TypeNormal {
		t.Errorf("expected default type 'normal', got %q", cl.Type)
	}
	for i, item := range cl.Items {
		if item.Type != ItemDo {
			t.Errorf("item %d: expected default type 'do', got %q", i, item.Type)
		}
		if item.ID == "" {
			t.Errorf("item %d: expected auto-generated ID, got empty string", i)
		}
	}
}

func TestLoad_autoIDsAreUniqueAcrossPhases(t *testing.T) {
	path := writeYAML(t, `
name: phased
phases:
  - name: A
    items:
      - label: Item one
      - label: Item two
  - name: B
    items:
      - label: Item three
      - label: Item four
`)
	cl, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	flat := cl.Flatten()
	seen := map[string]bool{}
	for _, fi := range flat {
		if seen[fi.Item.ID] {
			t.Errorf("duplicate auto-generated ID %q across phases", fi.Item.ID)
		}
		seen[fi.Item.ID] = true
	}
}

func TestLoad_invalidYAML(t *testing.T) {
	path := writeYAML(t, `not: valid: yaml: [`)
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoad_missingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/checklist.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoad_rejectsNoName(t *testing.T) {
	path := writeYAML(t, `
items:
  - id: i1
    label: Something
`)
	_, err := Load(path)
	if err == nil {
		t.Error("expected validation error for checklist with no name")
	}
}

func TestLoad_rejectsDuplicateIDs(t *testing.T) {
	path := writeYAML(t, `
name: bad
items:
  - id: same
    label: First
  - id: same
    label: Second
`)
	_, err := Load(path)
	if err == nil {
		t.Error("expected validation error for duplicate item IDs")
	}
}
