package ghscheduler

import (
	"testing"

	"github.com/octoberswimmer/p2-github-scheduler/p2"
	"github.com/octoberswimmer/p2/github"
)

func TestBuildReverseDependencies_BlockingCreatesBlockedBy(t *testing.T) {
	// Issue A blocks issue B - B should get A added to its blockedBy
	issues := map[string]p2.IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 1,
			Title:    "Blocker Task",
			State:    "open",
			Blocking: []github.IssueRef{
				{Owner: "owner", Repo: "repo", Number: 2}, // blocks issue 2
			},
		},
		"github.com/owner/repo/issues/2": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 2,
			Title:    "Blocked Task",
			State:    "open",
			// blockedBy is empty initially
		},
	}

	BuildReverseDependencies(issues)

	blockedIssue := issues["github.com/owner/repo/issues/2"]
	if len(blockedIssue.BlockedBy) != 1 {
		t.Errorf("expected 1 blockedBy entry, got %d", len(blockedIssue.BlockedBy))
	}

	if len(blockedIssue.BlockedBy) > 0 {
		ref := blockedIssue.BlockedBy[0]
		if ref.Owner != "owner" || ref.Repo != "repo" || ref.Number != 1 {
			t.Errorf("expected blockedBy to reference owner/repo#1, got %s/%s#%d", ref.Owner, ref.Repo, ref.Number)
		}
	}
}

func TestBuildReverseDependencies_CrossRepoBlocking(t *testing.T) {
	// Issue in repo A blocks issue in repo B
	issues := map[string]p2.IssueWithProject{
		"github.com/owner/repoA/issues/1": {
			Owner:    "owner",
			Repo:     "repoA",
			IssueNum: 1,
			Title:    "Blocker in Repo A",
			State:    "open",
			Blocking: []github.IssueRef{
				{Owner: "owner", Repo: "repoB", Number: 1}, // blocks issue in repo B
			},
		},
		"github.com/owner/repoB/issues/1": {
			Owner:    "owner",
			Repo:     "repoB",
			IssueNum: 1,
			Title:    "Blocked in Repo B",
			State:    "open",
		},
	}

	BuildReverseDependencies(issues)

	blockedIssue := issues["github.com/owner/repoB/issues/1"]
	if len(blockedIssue.BlockedBy) != 1 {
		t.Errorf("expected 1 blockedBy entry for cross-repo dependency, got %d", len(blockedIssue.BlockedBy))
	}

	if len(blockedIssue.BlockedBy) > 0 {
		ref := blockedIssue.BlockedBy[0]
		if ref.Owner != "owner" || ref.Repo != "repoA" || ref.Number != 1 {
			t.Errorf("expected blockedBy to reference owner/repoA#1, got %s/%s#%d", ref.Owner, ref.Repo, ref.Number)
		}
	}
}

func TestBuildReverseDependencies_NoDuplicates(t *testing.T) {
	// Issue already has blockedBy set - shouldn't add duplicate
	issues := map[string]p2.IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 1,
			Title:    "Blocker Task",
			State:    "open",
			Blocking: []github.IssueRef{
				{Owner: "owner", Repo: "repo", Number: 2},
			},
		},
		"github.com/owner/repo/issues/2": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 2,
			Title:    "Blocked Task",
			State:    "open",
			BlockedBy: []github.IssueRef{
				{Owner: "owner", Repo: "repo", Number: 1}, // already has the dependency
			},
		},
	}

	BuildReverseDependencies(issues)

	blockedIssue := issues["github.com/owner/repo/issues/2"]
	if len(blockedIssue.BlockedBy) != 1 {
		t.Errorf("expected 1 blockedBy entry (no duplicate), got %d", len(blockedIssue.BlockedBy))
	}
}
