package tui

import "github.com/charmbracelet/lipgloss"

var (
	subtle    = lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#5C5C5C"}
	highlight = lipgloss.AdaptiveColor{Light: "#1A1A1A", Dark: "#EEEEEE"}
	green     = lipgloss.AdaptiveColor{Light: "#2D9B4E", Dark: "#2ECC71"}
	yellow    = lipgloss.AdaptiveColor{Light: "#B8860B", Dark: "#F1C40F"}
	accent    = lipgloss.AdaptiveColor{Light: "#4A4A8A", Dark: "#7878C8"}
	dim       = lipgloss.AdaptiveColor{Light: "#C0C0C0", Dark: "#3A3A3A"}

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight)

	progressStyle = lipgloss.NewStyle().
			Foreground(subtle)

	phaseStyle = lipgloss.NewStyle().
			Foreground(accent)

	// Current item
	currentBulletStyle = lipgloss.NewStyle().
				Foreground(accent).
				Bold(true)

	labelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(highlight)

	responseStyle = lipgloss.NewStyle().
			Foreground(subtle).
			Italic(true)

	noteStyle = lipgloss.NewStyle().
			Foreground(subtle).
			Faint(true)

	// Completed item
	doneBulletStyle = lipgloss.NewStyle().
			Foreground(green)

	doneItemStyle = lipgloss.NewStyle().
			Foreground(subtle)

	// N/A item
	naStyle = lipgloss.NewStyle().
		Foreground(yellow)

	// Pending item
	pendingBulletStyle = lipgloss.NewStyle().
				Foreground(dim)

	pendingItemStyle = lipgloss.NewStyle().
				Foreground(dim)

	keyStyle = lipgloss.NewStyle().
			Foreground(subtle)

	doneStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(green)

	mutedStyle = lipgloss.NewStyle().
			Foreground(subtle)
)
