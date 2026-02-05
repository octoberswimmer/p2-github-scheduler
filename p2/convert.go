package p2

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/octoberswimmer/p2/planner"
	"github.com/octoberswimmer/p2/planner/lseq"
	"github.com/octoberswimmer/p2/recfile"
	"github.com/sirupsen/logrus"
)

type semver struct {
	major int
	minor int
	patch int
}

var semverPattern = regexp.MustCompile(`(?i)v?(\d+)\.(\d+)\.(\d+)`)

func parseSemver(s string) (semver, bool) {
	matches := semverPattern.FindStringSubmatch(s)
	if len(matches) != 4 {
		return semver{}, false
	}
	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return semver{}, false
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return semver{}, false
	}
	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return semver{}, false
	}
	return semver{major: major, minor: minor, patch: patch}, true
}

func compareSemver(a, b semver) int {
	if a.major != b.major {
		if a.major < b.major {
			return -1
		}
		return 1
	}
	if a.minor != b.minor {
		if a.minor < b.minor {
			return -1
		}
		return 1
	}
	if a.patch != b.patch {
		if a.patch < b.patch {
			return -1
		}
		return 1
	}
	return 0
}

type packageInfo struct {
	id         string
	hasDueDate bool
	dueDate    time.Time
	hasSemver  bool
	semver     semver
	firstOrder int
}

// IssuesToTasks converts GitHub issues to planner tasks.
// If privacy is non-nil, private repo information is redacted in log output.
func IssuesToTasks(issues map[string]IssueWithProject, privacy *PrivacyFilter) ([]planner.Task, []recfile.User, []SchedulingIssue) {
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

	// Build package ordering: due date (earliest first), then semver if present, then project order.
	packageInfos := make(map[string]*packageInfo)
	for i, ri := range sortedIssues {
		pkgID := ri.iwp.Milestone
		if pkgID == "" {
			continue
		}

		info, ok := packageInfos[pkgID]
		if !ok {
			info = &packageInfo{id: pkgID, firstOrder: i}
			if sv, ok := parseSemver(pkgID); ok {
				info.hasSemver = true
				info.semver = sv
			}
			packageInfos[pkgID] = info
		}

		if ri.iwp.MilestoneDueDate != nil {
			if !info.hasDueDate || ri.iwp.MilestoneDueDate.Before(info.dueDate) {
				info.hasDueDate = true
				info.dueDate = *ri.iwp.MilestoneDueDate
			}
		}
	}

	var orderedPackages []*packageInfo
	for _, info := range packageInfos {
		orderedPackages = append(orderedPackages, info)
	}
	sort.Slice(orderedPackages, func(i, j int) bool {
		a := orderedPackages[i]
		b := orderedPackages[j]

		if a.hasDueDate != b.hasDueDate {
			return a.hasDueDate
		}
		if a.hasDueDate && b.hasDueDate && !a.dueDate.Equal(b.dueDate) {
			return a.dueDate.Before(b.dueDate)
		}
		if a.hasSemver && b.hasSemver {
			if cmp := compareSemver(a.semver, b.semver); cmp != 0 {
				return cmp < 0
			}
		}
		if a.hasSemver != b.hasSemver {
			return a.hasSemver
		}
		return a.firstOrder < b.firstOrder
	})

	packageOrder := make(map[string]int, len(orderedPackages))
	for i, info := range orderedPackages {
		packageOrder[info.id] = i
	}
	unpackagedOrder := len(packageOrder)

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
		pkgID := iwp.Milestone
		if pkgID != "" {
			task.PackageID = pkgID
		}

		// Ensure milestone packages are ordered above unpackaged tasks
		if pkgID == "" {
			task.PackageOrder = unpackagedOrder
		} else if order, ok := packageOrder[pkgID]; ok {
			task.PackageOrder = order
		} else {
			task.PackageOrder = unpackagedOrder
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
				logDepID := depID
				logRepo := fmt.Sprintf("%s/%s", blocker.Owner, blocker.Repo)
				if privacy != nil {
					logDepID = privacy.RedactDepID(depID)
					logRepo = privacy.RedactRepo(blocker.Owner, blocker.Repo)
				}
				logrus.Warnf("Skipping dependency %s for %s: task not accessible (grant access to %s)", logDepID, task.ID, logRepo)
				continue
			}

			// Skip closed dependencies - they're already satisfied
			if strings.EqualFold(blockerIssue.State, "closed") {
				logrus.Debugf("Skipping dependency %s for %s: blocker is closed", depID, task.ID)
				continue
			}

			// Check if the dependency is on-hold
			if onHoldIssues[issueKey] {
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
