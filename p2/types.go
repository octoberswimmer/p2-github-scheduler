package p2

import (
	"time"

	"github.com/octoberswimmer/p2/github"
)

// IssueWithProject represents a GitHub issue with its project context
type IssueWithProject struct {
	Owner                string
	Repo                 string
	IssueNum             int
	Title                string
	Body                 string
	State                string
	Assignee             string
	Labels               []string
	Milestone            string
	Project              *github.ProjectItemInfo
	LowEstimate          float64
	HighEstimate         float64
	Order                int
	SchedulingStatus     string            // "On Hold", etc. from Scheduling Status field
	HasSchedulingDates   bool              // true if any scheduling date fields are set
	HasEstimates         bool              // true if Low Estimate or High Estimate are set
	BlockedBy            []github.IssueRef // issues that block this one
	Blocking             []github.IssueRef // issues that this one blocks
	InaccessibleBlockers int               // count of blockers from inaccessible repos
	IsDraft              bool              // true if this is a draft issue
	ProjectItemID        string            // project item node ID (used as ID for drafts)
}

// DateUpdate represents a date update to apply to an issue
type DateUpdate struct {
	Owner              string
	Repo               string
	RepoKey            string
	IssueNum           int
	Name               string
	Project            *github.ProjectItemInfo
	ExpectedStart      time.Time
	ExpectedCompletion time.Time
	Completion98       time.Time
	ClearDates         bool   // true if dates should be cleared (closed/on-hold tasks)
	ClearReason        string // "closed", "on hold", or "unschedulable"
}

// SchedulingIssue tracks problems that prevent an issue from being scheduled
type SchedulingIssue struct {
	IssueRef string   // e.g., "github.com/owner/repo/issues/123"
	IssueNum int      // issue number
	Owner    string   // repository owner
	Repo     string   // repository name
	Reason   string   // "cycle", "missing_dependency", "onhold_dependency", "missing_estimate", "invalid_estimate"
	Details  []string // cycle path or list of problematic deps
}
