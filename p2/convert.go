package p2

import (
	"fmt"
	"sort"
	"strings"

	"github.com/octoberswimmer/p2/planner"
	"github.com/octoberswimmer/p2/planner/lseq"
	"github.com/octoberswimmer/p2/recfile"
	"github.com/sirupsen/logrus"
)

// IssuesToTasks converts GitHub issues to planner tasks
func IssuesToTasks(issues map[string]IssueWithProject) ([]planner.Task, []recfile.User, []SchedulingIssue) {
	gen := lseq.NewGenerator("scheduler")
	userSet := make(map[string]bool)
	var tasks []planner.Task
	var schedIssues []SchedulingIssue

	// Build a set of on-hold issues for dependency checking
	onHoldIssues := make(map[string]bool)
	for ref, iwp := range issues {
		if iwp.SchedulingStatus == "On Hold" || iwp.IsDraft {
			onHoldIssues[ref] = true
		}
	}

	// Convert map to slice and sort by order to preserve GitHub Project ordering
	type refIssue struct {
		ref string
		iwp IssueWithProject
	}
	sortedIssues := make([]refIssue, 0, len(issues))
	for ref, iwp := range issues {
		sortedIssues = append(sortedIssues, refIssue{ref: ref, iwp: iwp})
	}
	sort.Slice(sortedIssues, func(i, j int) bool {
		return sortedIssues[i].iwp.Order < sortedIssues[j].iwp.Order
	})

	for i, ri := range sortedIssues {
		ref := ri.ref
		iwp := ri.iwp

		// Determine task ID based on whether it's a draft or regular issue
		var taskID string
		if iwp.IsDraft {
			taskID = fmt.Sprintf("draft:%s", iwp.ProjectItemID)
		} else {
			taskID = fmt.Sprintf("%s/%s#%d", iwp.Owner, iwp.Repo, iwp.IssueNum)
		}

		task := planner.Task{
			ID:       taskID,
			Sequence: lseq.SequentialString(i, "scheduler"),
			Name:     iwp.Title,
			Ref:      []string{ref},
			Done:     strings.EqualFold(iwp.State, "closed"),
		}
		if iwp.LowEstimate != nil {
			task.EstimateLow = *iwp.LowEstimate
		}
		if iwp.HighEstimate != nil {
			task.EstimateHigh = *iwp.HighEstimate
		}

		// Default estimates if not set
		if iwp.LowEstimate == nil && iwp.HighEstimate == nil && !task.Done {
			task.EstimateLow = 1
			task.EstimateHigh = 4
		}

		// Extract assignee (use "unassigned" for tasks with no assignee)
		if iwp.Assignee != "" {
			task.User = iwp.Assignee
			userSet[iwp.Assignee] = true
		} else {
			task.User = "unassigned"
			userSet["unassigned"] = true
		}

		// Draft issues are always on-hold
		if iwp.IsDraft {
			task.OnHold = true
		}

		// Check for on-hold via Scheduling Status field
		if iwp.SchedulingStatus == "On Hold" {
			task.OnHold = true
		}

		// Extract milestone as package
		if iwp.Milestone != "" {
			task.PackageID = iwp.Milestone
		}

		// Track scheduling issues for this task
		var missingDeps []string
		var onHoldDeps []string

		// Map blockedBy to DependsOn (only if the blocking task exists in our data)
		for _, blocker := range iwp.BlockedBy {
			depID := fmt.Sprintf("%s/%s#%d", blocker.Owner, blocker.Repo, blocker.Number)
			issueKey := fmt.Sprintf("github.com/%s/%s/issues/%d", blocker.Owner, blocker.Repo, blocker.Number)

			blockerIssue, exists := issues[issueKey]
			if !exists {
				// Missing dependency - not in the project
				missingDeps = append(missingDeps, depID)
				logrus.Warnf("Skipping dependency %s for %s: task not accessible (grant access to %s/%s)", depID, task.ID, blocker.Owner, blocker.Repo)
				continue
			}

			// Check if the dependency is on-hold (but not closed - closed deps are ok)
			if onHoldIssues[issueKey] && !strings.EqualFold(blockerIssue.State, "closed") {
				onHoldDeps = append(onHoldDeps, depID)
				logrus.Debugf("Dependency %s for %s is on-hold", depID, task.ID)
				// Don't add on-hold deps to DependsOn - they would block scheduling
				continue
			}

			task.DependsOn = append(task.DependsOn, depID)
			logrus.Debugf("Added dependency: %s depends on %s", task.ID, depID)
		}

		// Record scheduling issues for non-on-hold, non-closed tasks
		if !task.OnHold && !task.Done {
			if len(missingDeps) > 0 {
				schedIssues = append(schedIssues, SchedulingIssue{
					IssueRef: ref,
					IssueNum: iwp.IssueNum,
					Owner:    iwp.Owner,
					Repo:     iwp.Repo,
					Reason:   "missing_dependency",
					Details:  missingDeps,
				})
			}
			if len(onHoldDeps) > 0 {
				schedIssues = append(schedIssues, SchedulingIssue{
					IssueRef: ref,
					IssueNum: iwp.IssueNum,
					Owner:    iwp.Owner,
					Repo:     iwp.Repo,
					Reason:   "onhold_dependency",
					Details:  onHoldDeps,
				})
			}
			if iwp.InaccessibleBlockers > 0 {
				details := []string{fmt.Sprintf("%d blocker(s) from inaccessible repositories", iwp.InaccessibleBlockers)}
				schedIssues = append(schedIssues, SchedulingIssue{
					IssueRef: ref,
					IssueNum: iwp.IssueNum,
					Owner:    iwp.Owner,
					Repo:     iwp.Repo,
					Reason:   "inaccessible_dependency",
					Details:  details,
				})
			}
			// Check for missing estimates (nil means not set)
			var missingEstimates []string
			if iwp.LowEstimate == nil {
				missingEstimates = append(missingEstimates, "Low Estimate")
			}
			if iwp.HighEstimate == nil {
				missingEstimates = append(missingEstimates, "High Estimate")
			}
			if len(missingEstimates) > 0 {
				schedIssues = append(schedIssues, SchedulingIssue{
					IssueRef: ref,
					IssueNum: iwp.IssueNum,
					Owner:    iwp.Owner,
					Repo:     iwp.Repo,
					Reason:   "missing_estimate",
					Details:  missingEstimates,
				})
			}
			// Check for invalid estimates (high must be >= low, only when both are set)
			if iwp.LowEstimate != nil && iwp.HighEstimate != nil && *iwp.HighEstimate < *iwp.LowEstimate {
				schedIssues = append(schedIssues, SchedulingIssue{
					IssueRef: ref,
					IssueNum: iwp.IssueNum,
					Owner:    iwp.Owner,
					Repo:     iwp.Repo,
					Reason:   "invalid_estimate",
					Details:  []string{fmt.Sprintf("High Estimate (%.1f) must be greater than or equal to Low Estimate (%.1f)", *iwp.HighEstimate, *iwp.LowEstimate)},
				})
			}
		}

		tasks = append(tasks, task)
	}

	// Assign sequences properly
	for j := range tasks {
		seq, _ := gen.After(tasks[max(0, j-1)].Sequence)
		tasks[j].Sequence = seq
	}

	// Create users with default availability
	var users []recfile.User
	for username := range userSet {
		users = append(users, recfile.User{
			ID:             username,
			MondayHours:    8,
			TuesdayHours:   8,
			WednesdayHours: 8,
			ThursdayHours:  8,
			FridayHours:    8,
		})
	}

	// Add default user if no assignees
	if len(users) == 0 {
		users = append(users, recfile.User{
			ID:             "unassigned",
			MondayHours:    8,
			TuesdayHours:   8,
			WednesdayHours: 8,
			ThursdayHours:  8,
			FridayHours:    8,
		})
	}

	return tasks, users, schedIssues
}
