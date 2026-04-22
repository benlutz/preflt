package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/benlutz/preflt/internal/checklist"
	"github.com/benlutz/preflt/internal/format"
	"github.com/benlutz/preflt/internal/runner"
	"github.com/benlutz/preflt/internal/schedule"
	"github.com/benlutz/preflt/internal/store"
	"github.com/benlutz/preflt/internal/web"
)

var version = "0.3.0"

var rootCmd = &cobra.Command{
	Use:   "preflt",
	Short: "Pilot-style checklist runner",
	// No Args restriction — called with no args shows the startup screen.
	Version:       version,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return startupScreen(cmd)
	},
}

var runCmd = &cobra.Command{
	Use:   "run <name|path>",
	Short: "Run a checklist by name or path",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runner.Run(args[0])
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available checklists",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := checklist.ListPaths()
		if err != nil {
			return fmt.Errorf("scanning checklists: %w", err)
		}

		if len(paths) == 0 {
			fmt.Println("  No checklists found.")
			fmt.Println("  Place .yaml files in ~/.preflt/ or the current directory.")
			return nil
		}

		nameW := 22
		descW := 32

		fmt.Printf("\n  %-*s  %-*s  %s\n", nameW, "Name", descW, "Description", "Last run")
		fmt.Printf("  %s\n", strings.Repeat("─", nameW+descW+14))

		for _, path := range paths {
			cl, err := checklist.Load(path)
			if err != nil {
				continue
			}

			name := cl.Name
			if name == "" {
				name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			}

			desc := cl.Description
			if len(desc) > descW {
				desc = desc[:descW-1] + "…"
			}

			lastRun := "never"
			logs, _ := store.LoadHistory(cl.Name, 1)
			if len(logs) > 0 {
				lastRun = format.TimeAgo(logs[0].CompletedAt)
			}

			fmt.Printf("  %-*s  %-*s  %s\n", nameW, name, descW, desc, lastRun)
		}
		fmt.Println()
		return nil
	},
}

var historyCmd = &cobra.Command{
	Use:   "history <name>",
	Short: "Show past runs of a checklist",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		logs, err := store.LoadHistory(name, 10)
		if err != nil {
			return fmt.Errorf("loading history: %w", err)
		}

		fmt.Printf("\n  %s — last %d runs\n\n", name, 10)

		if len(logs) == 0 {
			fmt.Println("  No completed runs found.")
			fmt.Println()
			return nil
		}

		dateW := 20
		durW := 10
		statusW := 12

		fmt.Printf("  %-*s  %-*s  %-*s  %s\n", dateW, "Date", durW, "Duration", statusW, "Status", "By")
		fmt.Printf("  %s\n", strings.Repeat("─", dateW+durW+statusW+20))

		for _, log := range logs {
			date := log.CompletedAt.Format("2006-01-02 15:04")
			dur := format.Duration(log.CompletedAt.Sub(log.StartedAt))
			fmt.Printf("  %-*s  %-*s  %-*s  %s\n",
				dateW, date,
				durW, dur,
				statusW, log.Status,
				log.CompletedBy,
			)
		}
		fmt.Println()
		return nil
	},
}

var webCmd = &cobra.Command{
	Use:   "web <name|path>",
	Short: "Run a checklist in the browser",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		host, _ := cmd.Flags().GetString("host")
		return web.Serve(args[0], host, port)
	},
}

var scheduleCmd = &cobra.Command{
	Use:   "schedule [name]",
	Short: "Manage checklist schedules",
	Long: `Without a name: list all active schedules.
With a name: add or update a schedule for that checklist.
Flags determine the mode:
  --pending           show until completed once
  --from DATE         show on/after DATE (YYYY-MM-DD) until completed once
  --frequency FREQ    recurring: daily | weekly | monthly
  --on WEEKDAY        weekday for weekly schedules (e.g. monday)
  --period PERIOD     time-of-day hint: morning | afternoon | evening
  --cooldown DUR      min gap between runs, e.g. 7d or 12h
  --remove            delete the schedule for this checklist`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return listSchedules()
		}
		name := args[0]
		remove, _ := cmd.Flags().GetBool("remove")
		if remove {
			if err := store.RemoveSchedule(name); err != nil {
				return err
			}
			fmt.Printf("  Schedule for %q removed.\n", name)
			return nil
		}
		return addOrUpdateSchedule(cmd, name)
	},
}

func init() {
	webCmd.Flags().Int("port", 8080, "Port to listen on")
	webCmd.Flags().String("host", "localhost", "Host to bind to (use 0.0.0.0 for network access)")

	scheduleCmd.Flags().Bool("pending", false, "Show until completed once")
	scheduleCmd.Flags().String("from", "", "Show on/after this date (YYYY-MM-DD)")
	scheduleCmd.Flags().String("frequency", "", "Recurrence: daily | weekly | monthly")
	scheduleCmd.Flags().String("on", "", "Weekday for weekly schedules")
	scheduleCmd.Flags().String("period", "", "Time-of-day hint: morning | afternoon | evening")
	scheduleCmd.Flags().String("cooldown", "", "Minimum gap between runs, e.g. 7d or 12h")
	scheduleCmd.Flags().Bool("remove", false, "Remove the schedule for this checklist")
}

// Execute is the entry point called from main.
func Execute() {
	rootCmd.AddCommand(runCmd, listCmd, historyCmd, webCmd, scheduleCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// startupScreen shows due checklists and lets the user pick one to run.
func startupScreen(cmd *cobra.Command) error {
	paths, _ := checklist.ListPaths()
	due := schedule.Due(paths)

	if len(due) == 0 {
		// Nothing due — fall back to help.
		return cmd.Help()
	}

	greeting := schedule.Greeting(store.CompletedBy())
	fmt.Printf("\n  %s\n\n", greeting)
	fmt.Println("  Due checklists:")
	fmt.Println()

	for i, item := range due {
		period := ""
		if item.Period != "" {
			period = " · " + item.Period
		}
		fmt.Printf("  [%d] %-24s %s%s\n", i+1, item.Checklist.Name, item.Reason, period)
	}

	fmt.Println()
	fmt.Printf("  Pick a checklist [1-%d] or [s] skip: ", len(due))

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "s" || input == "skip" || input == "" {
		fmt.Println()
		return nil
	}

	// Parse a number.
	idx := 0
	if _, err := fmt.Sscanf(input, "%d", &idx); err != nil || idx < 1 || idx > len(due) {
		fmt.Println("  Invalid choice.")
		return nil
	}

	fmt.Println()
	return runner.Run(due[idx-1].Path)
}

// listSchedules prints all active schedules from schedules.json.
func listSchedules() error {
	entries, err := store.LoadSchedules()
	if err != nil {
		return fmt.Errorf("loading schedules: %w", err)
	}
	if len(entries) == 0 {
		fmt.Println("\n  No schedules configured.")
		fmt.Println("  Use: preflt schedule <name> --frequency daily")
		fmt.Println()
		return nil
	}

	nameW := 24
	modeW := 12

	fmt.Printf("\n  %-*s  %-*s  %s\n", nameW, "Name", modeW, "Mode", "Details")
	fmt.Printf("  %s\n", strings.Repeat("─", nameW+modeW+30))

	for _, e := range entries {
		details := scheduleDetails(e)
		fmt.Printf("  %-*s  %-*s  %s\n", nameW, e.Name, modeW, e.Mode, details)
	}
	fmt.Println()
	return nil
}

// scheduleDetails returns a compact summary of the schedule entry's parameters.
func scheduleDetails(e store.ScheduleEntry) string {
	var parts []string
	switch e.Mode {
	case "pending":
		parts = append(parts, "show until completed once")
	case "date":
		if e.From != "" {
			parts = append(parts, "from "+e.From)
		}
	case "recurring":
		if e.Frequency != "" {
			s := e.Frequency
			if e.On != "" {
				s += " (" + schedule.FormatDays(e.On) + ")"
			}
			parts = append(parts, s)
		}
		if e.Cooldown != "" {
			parts = append(parts, "cooldown "+e.Cooldown)
		}
	}
	if e.Period != "" {
		parts = append(parts, e.Period)
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, "  ·  ")
}

// addOrUpdateSchedule creates or updates a schedules.json entry for the given
// checklist name based on the provided flags. Any field not explicitly set by
// the user is filled from the checklist's YAML schedule block if one exists.
func addOrUpdateSchedule(cmd *cobra.Command, nameOrPath string) error {
	pending, _ := cmd.Flags().GetBool("pending")
	from, _ := cmd.Flags().GetString("from")
	frequency, _ := cmd.Flags().GetString("frequency")
	on, _ := cmd.Flags().GetString("on")
	period, _ := cmd.Flags().GetString("period")
	cooldown, _ := cmd.Flags().GetString("cooldown")

	// Load the checklist to get canonical name, path, and YAML schedule hint.
	name := nameOrPath
	path := ""
	var yamlHint *checklist.Schedule
	if cl, err := checklist.Load(nameOrPath); err == nil {
		name = cl.Name
		yamlHint = cl.Schedule
		if strings.HasPrefix(nameOrPath, "./") || strings.HasPrefix(nameOrPath, "/") ||
			strings.HasSuffix(nameOrPath, ".yaml") || strings.HasSuffix(nameOrPath, ".yml") {
			if abs, err := filepath.Abs(nameOrPath); err == nil {
				path = abs
			} else {
				path = nameOrPath
			}
		}
	}

	// Determine mode from explicit user flags first.
	userSetMode := cmd.Flags().Changed("pending") || cmd.Flags().Changed("from") || cmd.Flags().Changed("frequency")

	// Fill unset fields from YAML hint. Frequency is only inferred when the
	// user has not set any mode flag, so it can't override --pending or --from.
	var inferred []string
	if yamlHint != nil {
		if !userSetMode && !cmd.Flags().Changed("frequency") && yamlHint.Frequency != "" {
			frequency = yamlHint.Frequency
			inferred = append(inferred, "frequency="+frequency)
		}
		// on/period/cooldown are only meaningful for recurring; skip them for
		// other modes unless explicitly provided.
		if frequency != "" || cmd.Flags().Changed("frequency") {
			if !cmd.Flags().Changed("on") && yamlHint.On != "" {
				on = yamlHint.On
				inferred = append(inferred, "on="+on)
			}
			if !cmd.Flags().Changed("period") && yamlHint.Period != "" {
				period = yamlHint.Period
				inferred = append(inferred, "period="+period)
			}
			if !cmd.Flags().Changed("cooldown") && yamlHint.Cooldown != "" {
				cooldown = yamlHint.Cooldown
				inferred = append(inferred, "cooldown="+cooldown)
			}
		}
	}

	// Determine mode.
	mode := ""
	switch {
	case frequency != "":
		mode = "recurring"
	case from != "":
		mode = "date"
	case pending:
		mode = "pending"
	default:
		return fmt.Errorf("specify at least one of --pending, --from, or --frequency\n  See: preflt schedule --help")
	}

	// Validate from date if given.
	if from != "" {
		if _, err := time.Parse("2006-01-02", from); err != nil {
			return fmt.Errorf("--from must be a date in YYYY-MM-DD format, got %q", from)
		}
	}

	// Validate frequency.
	if frequency != "" {
		switch frequency {
		case "daily", "weekly", "monthly":
		default:
			return fmt.Errorf("--frequency must be daily, weekly, or monthly, got %q", frequency)
		}
	}

	entry := store.ScheduleEntry{
		Name:      name,
		Path:      path,
		Mode:      mode,
		From:      from,
		Frequency: frequency,
		On:        on,
		Period:    period,
		Cooldown:  cooldown,
		CreatedAt: time.Now(),
	}

	if err := store.SaveSchedule(entry); err != nil {
		return fmt.Errorf("saving schedule: %w", err)
	}

	fmt.Printf("\n  Schedule set for %q\n", name)
	fmt.Printf("  Mode: %s  %s\n", mode, scheduleDetails(entry))
	if len(inferred) > 0 {
		fmt.Printf("  (from YAML hint: %s)\n", strings.Join(inferred, ", "))
	}
	fmt.Println()
	return nil
}
