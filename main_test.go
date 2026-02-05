package main

import (
	"errors"
	"testing"

	"github.com/octoberswimmer/p2/github"
	"github.com/spf13/cobra"
)

func TestRun_IssueNotInProject_ReturnsNil(t *testing.T) {
	// Save original functions
	origLookup := lookupProjectForIssue
	defer func() { lookupProjectForIssue = origLookup }()

	// Mock the lookup to return an error (simulating issue not in project)
	lookupProjectForIssue = func(accessToken string, info *github.URLInfo) (*github.URLInfo, error) {
		return nil, errors.New("issue #16 is not in any project")
	}

	// Set up environment
	t.Setenv("P2_LICENSE_KEY", `{"t":"test-token"}`)

	// Create a command with the issue URL
	cmd := &cobra.Command{}
	args := []string{"https://github.com/owner/repo/issues/16"}

	err := run(cmd, args)

	// Should return nil (success), not an error
	if err != nil {
		t.Errorf("expected nil error when issue is not in project, got: %v", err)
	}
}

func TestRun_IssueInProject_FetchesProject(t *testing.T) {
	// Save original functions
	origLookup := lookupProjectForIssue
	origFetch := fetchProjectItems
	defer func() {
		lookupProjectForIssue = origLookup
		fetchProjectItems = origFetch
	}()

	// Mock the lookup to return a project
	lookupProjectForIssue = func(accessToken string, info *github.URLInfo) (*github.URLInfo, error) {
		return &github.URLInfo{
			Owner:      "org",
			IsOrg:      true,
			IsProject:  true,
			ProjectNum: 1,
		}, nil
	}

	// Mock fetch to return empty issues (so we exit early)
	fetchProjectItems = func(accessToken string, info *github.URLInfo) (map[string]github.IssueWithProject, error) {
		return nil, nil
	}

	// Set up environment
	t.Setenv("P2_LICENSE_KEY", `{"t":"test-token"}`)

	cmd := &cobra.Command{}
	args := []string{"https://github.com/owner/repo/issues/16"}

	err := run(cmd, args)

	// Should return nil (success) - we exit early due to no issues
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

func TestRun_RepoURL_FetchesViaProjects(t *testing.T) {
	// Save original function
	origFetch := fetchRepoIssuesViaProjects
	defer func() { fetchRepoIssuesViaProjects = origFetch }()

	// Mock fetch to return empty issues
	fetchRepoIssuesViaProjects = func(accessToken string, info *github.URLInfo) (map[string]github.IssueWithProject, error) {
		return nil, nil
	}

	// Set up environment
	t.Setenv("P2_LICENSE_KEY", `{"t":"test-token"}`)

	cmd := &cobra.Command{}
	args := []string{"https://github.com/owner/repo"}

	err := run(cmd, args)

	// Should return nil (success) - no issues to schedule
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}
