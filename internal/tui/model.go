package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/benlutz/preflt/internal/checklist"
	"github.com/benlutz/preflt/internal/format"
	"github.com/benlutz/preflt/internal/store"
)

type screen int

const (
	screenList      screen = iota // full checklist view, steps through items
	screenInput                   // text input overlay for type:check items
	screenCondition               // yes/no prompt for condition items
	screenQuit                    // "Save progress? [y/n]"
	screenDone                    // completion summary
)

// Model is the Bubbletea model for the checklist TUI.
type Model struct {
	Checklist *checklist.Checklist
	State     *store.RunState
	Items     []checklist.FlatItem
	Cursor    int
	Screen    screen
	Input     textinput.Model
	Width     int
	Height    int
	Done        bool
	Quitting    bool
	Saved       bool   // true only if state was actually written to disk on quit
	TriggerNext string // checklist name to start after this run (set by condition or on_complete)
	Aborted     bool   // true when abort:true branch was taken

	// onItemComplete is called asynchronously after each item is confirmed.
	// Provided by the runner; nil means no item-level automations.
	onItemComplete func(itemIdx int, value string)
}

// New creates a new Model for the given checklist and run state.
// onItemComplete is called in a background goroutine when each item is
// confirmed — use it to fire item-level automations. Pass nil to skip.
func New(cl *checklist.Checklist, state *store.RunState, onItemComplete func(int, string)) Model {
	ti := textinput.New()
	ti.Placeholder = "type your answer..."
	ti.CharLimit = 256

	return Model{
		Checklist:      cl,
		State:          state,
		Items:          cl.Flatten(),
		Cursor:         state.CurrentIndex,
		Screen:         screenList,
		Input:          ti,
		Width:          80,
		Height:         24,
		onItemComplete: onItemComplete,
	}
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.Screen {
		case screenList:
			return m.updateList(msg)
		case screenInput:
			return m.updateInput(msg)
		case screenCondition:
			return m.updateCondition(msg)
		case screenQuit:
			return m.updateQuit(msg)
		case screenDone:
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		// Hard interrupt: save whatever progress exists.
		if m.Cursor > 0 {
			_ = store.SaveState(m.State)
			m.Saved = true
		}
		m.Quitting = true
		return m, tea.Quit

	case "q":
		if m.Cursor > 0 {
			// There's progress — ask whether to save it.
			m.Screen = screenQuit
			return m, nil
		}
		// Nothing done yet, quit without saving.
		m.Quitting = true
		return m, tea.Quit

	case "enter":
		if m.Cursor >= len(m.Items) {
			return m, nil
		}
		item := m.Items[m.Cursor].Item
		if item.Condition != nil {
			m.Screen = screenCondition
			return m, nil
		}
		if item.Type == checklist.ItemCheck {
			m.Input.Reset()
			m.Input.Focus()
			m.Screen = screenInput
			return m, textinput.Blink
		}
		var cmd tea.Cmd
		m, cmd = m.confirmCurrent("")
		return m, cmd

	case "n":
		if m.Cursor < len(m.Items) && m.Items[m.Cursor].Item.NAAllowed {
			m = m.markNA()
		}
		return m, nil
	}

	return m, nil
}

func (m Model) updateInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		val := strings.TrimSpace(m.Input.Value())
		m.Input.Blur()
		m.Screen = screenList
		var cmd tea.Cmd
		m, cmd = m.confirmCurrent(val)
		return m, cmd

	case "esc":
		m.Input.Blur()
		m.Screen = screenList
		return m, nil
	}

	var cmd tea.Cmd
	m.Input, cmd = m.Input.Update(msg)
	return m, cmd
}

func (m Model) updateCondition(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.Cursor >= len(m.Items) {
		return m, nil
	}
	cond := m.Items[m.Cursor].Item.Condition
	switch msg.String() {
	case "y", "Y":
		return m.applyBranch(cond.IfYes, "yes")
	case "n", "N":
		return m.applyBranch(cond.IfNo, "no")
	case "ctrl+c", "q":
		if m.Cursor > 0 {
			m.Screen = screenQuit
			return m, nil
		}
		m.Quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// applyBranch acts on a ConditionBranch after the user answers yes or no.
func (m Model) applyBranch(branch *checklist.ConditionBranch, answer string) (Model, tea.Cmd) {
	if branch == nil {
		// No branch defined for this answer — treat as a plain confirm.
		m.Screen = screenList
		return m.confirmCurrent(answer)
	}

	if branch.Skip {
		// Mark item as N/A and continue.
		m = m.markNA()
		m.Screen = screenList
		return m, nil
	}

	// Confirm the condition item with the answer as its value.
	var cmd tea.Cmd
	m, cmd = m.confirmCurrent(answer)

	if branch.TriggerChecklist != "" {
		m.TriggerNext = branch.TriggerChecklist
		if branch.Abort {
			m.Aborted = true
			m.Done = true
			m.Screen = screenDone
		}
	}

	if m.Screen != screenDone {
		m.Screen = screenList
	}
	return m, cmd
}

func (m Model) updateQuit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter", "ctrl+c":
		_ = store.SaveState(m.State)
		m.Saved = true
		m.Quitting = true
		return m, tea.Quit

	case "n", "N", "q", "esc":
		_ = store.DiscardState(m.State)
		m.Quitting = true
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) confirmCurrent(value string) (Model, tea.Cmd) {
	if m.Cursor >= len(m.Items) {
		return m, nil
	}

	now := time.Now()
	idx := m.Cursor

	if idx < len(m.State.Items) {
		m.State.Items[idx].Status = store.StatusChecked
		m.State.Items[idx].Value = value
		m.State.Items[idx].CheckedAt = &now
	}

	m.Cursor++
	m.State.CurrentIndex = m.Cursor
	_ = store.SaveState(m.State)

	if m.Cursor >= len(m.Items) {
		m.Done = true
		m.Screen = screenDone
	}

	// Fire item-level automations in the background without blocking the UI.
	var cmd tea.Cmd
	if m.onItemComplete != nil {
		cb := m.onItemComplete
		cmd = func() tea.Msg {
			cb(idx, value)
			return nil
		}
	}

	return m, cmd
}

func (m Model) markNA() Model {
	if m.Cursor >= len(m.Items) {
		return m
	}

	now := time.Now()
	idx := m.Cursor

	if idx < len(m.State.Items) {
		m.State.Items[idx].Status = store.StatusNA
		m.State.Items[idx].CheckedAt = &now
	}

	m.Cursor++
	m.State.CurrentIndex = m.Cursor
	_ = store.SaveState(m.State)

	if m.Cursor >= len(m.Items) {
		m.Done = true
		m.Screen = screenDone
	}

	return m
}

// View implements tea.Model.
func (m Model) View() string {
	switch m.Screen {
	case screenDone:
		return m.viewDone()
	case screenQuit:
		return m.viewQuit()
	default:
		return m.viewList() // screenList, screenInput, screenCondition all use list view
	}
}

// renderedLine is a single line of output with metadata for scrolling.
type renderedLine struct {
	text      string
	isCurrent bool // true for the first line of the current item
}

// viewList renders the full checklist with the current item expanded.
func (m Model) viewList() string {
	var lines []renderedLine
	add := func(text string, cur bool) {
		lines = append(lines, renderedLine{text, cur})
	}

	lastPhase := ""

	for i, fi := range m.Items {
		item := fi.Item
		isCurrent := i == m.Cursor

		// Phase separator whenever the phase name changes.
		if fi.PhaseName != "" && fi.PhaseName != lastPhase {
			lastPhase = fi.PhaseName
			add(m.renderPhaseRuler(fi.PhaseName), false)
		}

		var status store.ItemStatus
		val := ""
		if i < len(m.State.Items) {
			status = m.State.Items[i].Status
			val = m.State.Items[i].Value
		}

		switch {
		case isCurrent:
			add(currentBulletStyle.Render("  ●  ")+labelStyle.Render(item.Label), true)
			if item.Response != "" {
				add("       "+responseStyle.Render("→ "+item.Response), false)
			}
			// Input field appears inline when in input mode.
			if m.Screen == screenInput {
				add("       "+m.Input.View(), false)
			}
			if item.Note != "" {
				add("       "+noteStyle.Render("note: "+item.Note), false)
			}
			add("", false)

		case status == store.StatusChecked:
			suffix := ""
			if val != "" {
				suffix = "  " + mutedStyle.Render(val)
			}
			add(doneBulletStyle.Render("  ✓  ")+doneItemStyle.Render(item.Label)+suffix, false)

		case status == store.StatusNA:
			add(naStyle.Render("  —  ")+doneItemStyle.Render(item.Label)+" "+naStyle.Render("(N/A)"), false)

		default: // pending
			add(pendingBulletStyle.Render("  ○  ")+pendingItemStyle.Render(item.Label), false)
		}
	}

	// Header: 2 lines. Footer: 2 lines (blank + keys). Rest is content.
	const headerLines = 2
	const footerLines = 2
	available := m.Height - headerLines - footerLines
	if available < 4 {
		available = 4
	}

	// Find which line the current item starts on.
	currentLineIdx := -1
	for i, l := range lines {
		if l.isCurrent {
			currentLineIdx = i
			break
		}
	}

	// Scroll so the current item sits ~1/3 from the top of the visible window.
	offset := 0
	if currentLineIdx >= 0 && len(lines) > available {
		ideal := currentLineIdx - available/3
		if ideal < 0 {
			ideal = 0
		}
		maxOffset := len(lines) - available
		if ideal > maxOffset {
			ideal = maxOffset
		}
		offset = ideal
	}

	// Render header row.
	total := len(m.Items)
	current := m.Cursor + 1
	if m.Cursor >= total {
		current = total
	}

	progressText := progressStyle.Render(fmt.Sprintf("%d / %d", current, total))
	nameText := headerStyle.Render("  " + m.Checklist.Name)
	gap := m.Width - lipgloss.Width(nameText) - lipgloss.Width(progressText) - 2
	if gap < 1 {
		gap = 1
	}

	var b strings.Builder
	b.WriteString(nameText + strings.Repeat(" ", gap) + progressText + "\n\n")

	// Render visible content lines.
	end := offset + available
	if end > len(lines) {
		end = len(lines)
	}
	for _, l := range lines[offset:end] {
		b.WriteString(l.text + "\n")
	}

	// Render footer keybindings.
	keys := ""
	if m.Cursor < len(m.Items) {
		item := m.Items[m.Cursor].Item
		if item.Condition != nil || m.Screen == screenCondition {
			keys = "  [y] yes  [n] no  [q] quit"
		} else {
			keys = "  [enter] confirm"
			if item.NAAllowed {
				keys += "  [n] N/A"
			}
			keys += "  [q] quit"
		}
	}
	b.WriteString("\n" + keyStyle.Render(keys) + "\n")

	return b.String()
}

func (m Model) viewQuit() string {
	completed := 0
	for _, is := range m.State.Items {
		if is.Status != store.StatusPending {
			completed++
		}
	}

	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString("  " + headerStyle.Render(m.Checklist.Name) + "\n\n")
	b.WriteString(mutedStyle.Render(fmt.Sprintf("  %d / %d items done", completed, len(m.Items))))
	b.WriteString("\n\n")
	b.WriteString("  " + labelStyle.Render("Save progress?") + "  " + keyStyle.Render("[y] save  [n] discard") + "\n")
	return b.String()
}

func (m Model) viewDone() string {
	checked := 0
	naCount := 0
	for _, is := range m.State.Items {
		switch is.Status {
		case store.StatusChecked:
			checked++
		case store.StatusNA:
			naCount++
		}
	}

	elapsed := time.Since(m.State.StartedAt)
	stats := fmt.Sprintf("     %d items confirmed", checked)
	if naCount > 0 {
		stats += fmt.Sprintf("  ·  %d N/A", naCount)
	}
	stats += "  ·  " + format.Duration(elapsed)

	var b strings.Builder
	b.WriteString("\n\n")

	if m.Aborted && m.TriggerNext != "" {
		b.WriteString("  " + naStyle.Render("↪  "+m.Checklist.Name+" aborted") + "\n\n")
		b.WriteString(noteStyle.Render("     starting → "+m.TriggerNext) + "\n\n")
	} else if m.TriggerNext != "" {
		b.WriteString("  " + doneStyle.Render("✓  "+m.Checklist.Name+" complete") + "\n\n")
		b.WriteString(noteStyle.Render(stats) + "\n\n")
		b.WriteString(noteStyle.Render("     next → "+m.TriggerNext) + "\n\n")
	} else {
		b.WriteString("  " + doneStyle.Render("✓  "+m.Checklist.Name+" complete") + "\n\n")
		b.WriteString(noteStyle.Render(stats) + "\n\n")
	}

	b.WriteString(keyStyle.Render("  [enter] continue") + "\n")
	return b.String()
}

func (m Model) renderPhaseRuler(phase string) string {
	rulerLen := m.Width - 8 - lipgloss.Width(phase)
	if rulerLen < 2 {
		rulerLen = 2
	}
	return phaseStyle.Render("  ── " + phase + " " + strings.Repeat("─", rulerLen))
}

