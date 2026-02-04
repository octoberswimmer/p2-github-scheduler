package p2

import (
	"fmt"
	"strings"
	"time"

	"github.com/octoberswimmer/p2/planner"
	"github.com/sirupsen/logrus"
)

// sameDate returns true if the existing date matches the new date (comparing only the date portion)
func sameDate(existing *time.Time, new time.Time) bool {
	if existing == nil {
		return new.IsZero()
	}
	return existing.Format("2006-01-02") == new.Format("2006-01-02")
}

// DetectAtRiskIssues identifies issues where expected completion is after the due date
func DetectAtRiskIssues(updates []DateUpdate, issues map[string]IssueWithProject) []SchedulingIssue {
	var atRiskIssues []SchedulingIssue

	for _, update := range updates {
		// Skip updates that are clearing dates (closed/on-hold)
		if update.ClearDates {
			continue
		}

		// Find the corresponding issue
		ref := fmt.Sprintf("github.com/%s/%s/issues/%d", update.Owner, update.Repo, update.IssueNum)
		iwp, ok := issues[ref]
		if !ok {
			continue
		}

		// Skip if no due date set
		if iwp.DueDate == nil {
			continue
		}

		// Check if expected completion is after due date
		if !update.ExpectedCompletion.IsZero() && update.ExpectedCompletion.After(*iwp.DueDate) {
			details := []string{
				fmt.Sprintf("Due Date: %s", iwp.DueDate.Format("2006-01-02")),
				fmt.Sprintf("Expected Completion: %s", update.ExpectedCompletion.Format("2006-01-02")),
			}
			atRiskIssues = append(atRiskIssues, SchedulingIssue{
				IssueRef: ref,
				IssueNum: iwp.IssueNum,
				Owner:    iwp.Owner,
				Repo:     iwp.Repo,
				Reason:   "at_risk",
				Details:  details,
			})
		}
	}

	return atRiskIssues
}

// ExtractCycleIssues checks scheduler results for dependency cycles and adds them to scheduling issues
func ExtractCycleIssues(entries planner.ScheduledEntries, issues map[string]IssueWithProject, existing []SchedulingIssue) []SchedulingIssue {
	// Build a set of issues that already have scheduling issues (avoid duplicates)
	hasIssue := make(map[string]bool)
	for _, si := range existing {
		hasIssue[si.IssueRef] = true
	}

	for _, entry := range entries.Entries {
		if entry.IsPackage || len(entry.Cycle) == 0 {
			continue
		}

		// Find the issue for this entry
		for ref, iwp := range issues {
			taskID := fmt.Sprintf("%s/%s#%d", iwp.Owner, iwp.Repo, iwp.IssueNum)
			if taskID == entry.ID && !hasIssue[ref] {
				existing = append(existing, SchedulingIssue{
					IssueRef: ref,
					IssueNum: iwp.IssueNum,
					Owner:    iwp.Owner,
					Repo:     iwp.Repo,
					Reason:   "cycle",
					Details:  entry.Cycle,
				})
				hasIssue[ref] = true
				break
			}
		}
	}

	return existing
}

// PrepareUpdates determines date updates to apply based on scheduling results
func PrepareUpdates(ganttData planner.GanttData, issues map[string]IssueWithProject, unschedulable map[string]bool) []DateUpdate {
	var updates []DateUpdate

	// Track which issues we've processed for clearing
	// Use full task ID to avoid collisions between repos with same issue numbers
	processed := make(map[string]bool)

	// First pass: check all issues directly for on-hold or closed status
	// This doesn't depend on the scheduler - just GitHub data
	for _, iwp := range issues {
		if iwp.Project == nil {
			continue
		}

		taskID := fmt.Sprintf("%s/%s#%d", iwp.Owner, iwp.Repo, iwp.IssueNum)

		// Check for on-hold via Scheduling Status field
		isOnHold := iwp.SchedulingStatus == "On Hold"
		isClosed := strings.EqualFold(iwp.State, "closed")

		if isOnHold || isClosed {
			// Mark as processed so second pass skips it
			processed[taskID] = true

			// On-hold: only clear if dates are set (estimates remain)
			// Closed: clear if dates OR estimates are set
			if isOnHold && !iwp.HasSchedulingDates {
				continue
			}
			if isClosed && !iwp.HasSchedulingDates && iwp.LowEstimate == nil && iwp.HighEstimate == nil {
				continue
			}
			reason := "on hold"
			if isClosed {
				reason = "closed"
			}
			update := DateUpdate{
				Owner:       iwp.Owner,
				Repo:        iwp.Repo,
				RepoKey:     fmt.Sprintf("%s/%s", iwp.Owner, iwp.Repo),
				IssueNum:    iwp.IssueNum,
				Name:        iwp.Title,
				Project:     iwp.Project,
				ClearDates:  true,
				ClearReason: reason,
			}
			updates = append(updates, update)
		}
	}

	// Build a map from task ID to issue for gantt bar lookup
	taskToIssue := make(map[string]IssueWithProject)
	for _, iwp := range issues {
		taskID := fmt.Sprintf("%s/%s#%d", iwp.Owner, iwp.Repo, iwp.IssueNum)
		taskToIssue[taskID] = iwp
	}

	// Second pass: handle unschedulable issues (clear their dates)
	for ref, iwp := range issues {
		if iwp.Project == nil {
			continue
		}

		taskID := fmt.Sprintf("%s/%s#%d", iwp.Owner, iwp.Repo, iwp.IssueNum)

		// Skip if already processed (on-hold or closed)
		if processed[taskID] {
			continue
		}

		// Check if this issue has scheduling problems
		if unschedulable[ref] && iwp.HasSchedulingDates {
			processed[taskID] = true
			update := DateUpdate{
				Owner:       iwp.Owner,
				Repo:        iwp.Repo,
				RepoKey:     fmt.Sprintf("%s/%s", iwp.Owner, iwp.Repo),
				IssueNum:    iwp.IssueNum,
				Name:        iwp.Title,
				Project:     iwp.Project,
				ClearDates:  true,
				ClearReason: "unschedulable",
			}
			updates = append(updates, update)
		}
	}

	// Third pass: check gantt bars for active tasks that need date updates
	for _, bar := range ganttData.Bars {
		if bar.IsPackage {
			continue
		}

		iwp, ok := taskToIssue[bar.ID]
		if !ok {
			logrus.Debugf("No issue found for task %s", bar.ID)
			continue
		}

		if iwp.Project == nil {
			logrus.Debugf("Issue %s is not in a project", bar.ID)
			continue
		}

		// Skip if already processed (on-hold, closed, or unschedulable)
		if processed[bar.ID] {
			continue
		}

		// Skip issues with scheduling problems (don't write dates)
		ref := fmt.Sprintf("github.com/%s/%s/issues/%d", iwp.Owner, iwp.Repo, iwp.IssueNum)
		if unschedulable[ref] {
			continue
		}

		// Skip if all dates are unchanged
		if sameDate(iwp.ExpectedStart, bar.ExpStartDate) &&
			sameDate(iwp.ExpectedCompletion, bar.MeanDate) &&
			sameDate(iwp.Completion98, bar.End98Date) {
			logrus.Debugf("Dates unchanged for %s, skipping", bar.ID)
			continue
		}

		update := DateUpdate{
			Owner:              iwp.Owner,
			Repo:               iwp.Repo,
			RepoKey:            fmt.Sprintf("%s/%s", iwp.Owner, iwp.Repo),
			IssueNum:           iwp.IssueNum,
			Name:               bar.Name,
			Project:            iwp.Project,
			ExpectedStart:      bar.ExpStartDate,
			ExpectedCompletion: bar.MeanDate,
			Completion98:       bar.End98Date,
		}

		updates = append(updates, update)
	}

	return updates
}
