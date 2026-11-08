package cli

import (
	"fmt"
	"strings"
)

type PlanHeader struct {
	Title string
	Env   string
	Extra []PlanHeaderExtra
}

type PlanHeaderExtra struct {
	Label string
	Value string
}

type PlanRow struct {
	Action  string
	Name    string
	Details string
	Server  string
}

func DisplayPlanHeader(h PlanHeader) {
	fmt.Println(h.Title)
	fmt.Printf("ENV:  %s\n", h.Env)
	for _, e := range h.Extra {
		fmt.Printf("%s:  %s\n", e.Label, e.Value)
	}
	fmt.Println()
}

func DisplayPlanTable4Col(actionLabel, nameLabel, serverLabel, detailsLabel string, rows []PlanRow) {
	fmt.Printf("%-8s %-16s %-12s %s\n", actionLabel, nameLabel, serverLabel, detailsLabel)
	for _, r := range rows {
		fmt.Printf("%-8s %-16s %-12s %s\n", r.Action, r.Name, r.Server, r.Details)
	}
}

func DisplayPlanTable3Col(actionLabel, nameLabel, detailsLabel string, rows []PlanRow) {
	fmt.Printf("%-8s %-20s %s\n", actionLabel, nameLabel, detailsLabel)
	for _, r := range rows {
		fmt.Printf("%-8s %-20s %s\n", r.Action, r.Name, r.Details)
	}
}

func DisplaySummary(text string) {
	fmt.Printf("\n%s\n", text)
}

func DisplayDryRun() {
	fmt.Println("\n[DRY RUN] No changes were made.")
}

func DisplayNoChanges(dryRun, force bool) {
	fmt.Println("No changes detected.")
	if !dryRun && !force {
		fmt.Println()
		fmt.Println("[INFO] Use --force to deploy even without configuration changes.")
	}
}

func DisplayCancelled() {
	fmt.Println("\n[INFO] Operation cancelled by user.")
}

func formatSummary(created, updated, deleted int, forced bool) string {
	s := fmt.Sprintf("SUMMARY: %d created, %d updated, %d deleted", created, updated, deleted)
	if forced {
		s += " (forced)"
	}
	return s
}

func formatSummaryCount(noun string, count int) string {
	return fmt.Sprintf("SUMMARY: %d %s", count, noun)
}

func buildPlanTitle(command string, dryRun, force bool) string {
	title := "PLAN: " + command
	var tags []string
	if force {
		tags = append(tags, "forced")
	}
	if dryRun {
		tags = append(tags, "dry-run")
	}
	if len(tags) > 0 {
		title += " (" + strings.Join(tags, ", ") + ")"
	}
	return title
}
