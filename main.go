package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/joho/godotenv"
	"github.com/octoberswimmer/p2-github-scheduler/ghscheduler"
	"github.com/octoberswimmer/p2-github-scheduler/p2"
	"github.com/octoberswimmer/p2/github"
	"github.com/octoberswimmer/p2/planner"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	debug   bool
	dryRun  bool
	rootCmd = &cobra.Command{
		Use:   "p2-github-scheduler <github-url>",
		Short: "Reschedule GitHub issues using p2's scheduling algorithm",
		Long: `Fetches GitHub issues from a repository or project, runs p2's
scheduling algorithm, and updates the calculated date fields
(Expected Start, Expected Completion, 98% Completion) in GitHub Projects.

Accepts:
  - Project URL: https://github.com/orgs/org/projects/1
  - Repository URL: https://github.com/owner/repo
  - Issue URL: https://github.com/owner/repo/issues/123
  - Short form: owner/repo`,
		Args: cobra.ExactArgs(1),
		RunE: run,
	}
)

func init() {
	// Load .env file if present (same as p2)
	godotenv.Load()

	rootCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be updated without making changes")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.WarnLevel)
	}

	url := args[0]

	// Authenticate with GitHub
	var accessToken string

	// First check for GITHUB_TOKEN environment variable (for CI)
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		logrus.Debug("Using GITHUB_TOKEN from environment")
		accessToken = token
	} else {
		// Fall back to stored auth (auto-refreshes if expired) or device flow
		auth, err := github.LoadAndRefreshAuth()
		if err != nil {
			fmt.Println("No stored GitHub authentication found. Starting device flow...")
			auth, err = runDeviceFlow()
			if err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
		}

		// Verify token (in case refresh token also expired)
		if err := github.VerifyToken(auth.AccessToken); err != nil {
			fmt.Println("Stored token is invalid. Starting device flow...")
			auth, err = runDeviceFlow()
			if err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
		}
		accessToken = auth.AccessToken
	}

	// Parse the URL to determine what we're working with
	urlInfo, err := ghscheduler.ParseGitHubURL(url)
	if err != nil {
		return fmt.Errorf("invalid GitHub URL: %w", err)
	}

	var allIssues map[string]p2.IssueWithProject

	if urlInfo.IsProject {
		// Fetch issues directly from the project
		fmt.Printf("Fetching items from project %s #%d...\n", urlInfo.Owner, urlInfo.ProjectNum)
		allIssues, err = ghscheduler.FetchProjectItems(accessToken, urlInfo)
		if err != nil {
			return err
		}
	} else {
		// Fetch issues from the repository
		fmt.Printf("Scheduling tasks for %s/%s\n", urlInfo.Owner, urlInfo.Repo)
		allIssues, err = ghscheduler.FetchRepoIssues(accessToken, urlInfo)
		if err != nil {
			return err
		}
	}

	if len(allIssues) == 0 {
		fmt.Println("No issues to schedule")
		return nil
	}

	// Convert issues to p2 tasks
	fmt.Println("Converting to p2 tasks...")
	tasks, users, schedIssues := p2.IssuesToTasks(allIssues)
	fmt.Printf("Created %d tasks with %d users\n", len(tasks), len(users))

	if len(tasks) == 0 {
		fmt.Println("No tasks to schedule")
		return nil
	}

	// Run the scheduler
	fmt.Println("Running scheduler...")
	for _, t := range tasks {
		if len(t.DependsOn) > 0 {
			logrus.Debugf("Task %s (user=%q, done=%v, onhold=%v) depends on: %v", t.ID, t.User, t.Done, t.OnHold, t.DependsOn)
		}
	}
	base := time.Now()
	entries := planner.ScheduleWithUsers(tasks, users)

	// Extract cycle information from scheduler results
	schedIssues = p2.ExtractCycleIssues(entries, allIssues, schedIssues)

	ganttData, err := planner.ComputeGanttData(entries, tasks, true, base, users)
	if err != nil {
		return fmt.Errorf("scheduling failed: %w", err)
	}

	// Build set of issues with scheduling problems
	unschedulableIssues := make(map[string]bool)
	for _, si := range schedIssues {
		unschedulableIssues[si.IssueRef] = true
	}

	// Prepare updates
	updates := p2.PrepareUpdates(ganttData, allIssues, unschedulableIssues)

	// Print scheduling issues
	if len(schedIssues) > 0 {
		fmt.Printf("\nFound %d issues with scheduling problems:\n", len(schedIssues))
		for _, si := range schedIssues {
			fmt.Printf("  %s/%s #%d: %s\n", si.Owner, si.Repo, si.IssueNum, si.Reason)
			for _, detail := range si.Details {
				fmt.Printf("       - %s\n", detail)
			}
		}
	}

	if len(updates) == 0 && len(schedIssues) == 0 {
		fmt.Println("No date changes needed")
		return nil
	}

	if len(updates) > 0 {
		fmt.Printf("\nFound %d tasks with date changes:\n", len(updates))
		for _, u := range updates {
			fmt.Printf("  %s #%d %s\n", u.RepoKey, u.IssueNum, u.Name)
			if u.ClearDates {
				if u.ClearReason == "closed" {
					fmt.Println("       (clearing dates and estimates - task is closed)")
				} else if u.ClearReason == "unschedulable" {
					fmt.Println("       (clearing dates - has scheduling issues)")
				} else {
					fmt.Println("       (clearing dates - task is on hold)")
				}
			} else {
				if !u.ExpectedStart.IsZero() {
					fmt.Printf("       Expected Start: %s\n", u.ExpectedStart.Format("2006-01-02"))
				}
				if !u.ExpectedCompletion.IsZero() {
					fmt.Printf("       Expected Completion: %s\n", u.ExpectedCompletion.Format("2006-01-02"))
				}
				if !u.Completion98.IsZero() {
					fmt.Printf("       98%% Completion: %s\n", u.Completion98.Format("2006-01-02"))
				}
			}
		}
	}

	if dryRun {
		fmt.Println("\nDry run - no changes made")
		return nil
	}

	// Apply updates to GitHub
	fmt.Println("\nUpdating GitHub...")
	for _, u := range updates {
		client := github.NewClient(accessToken, &github.GitHubRepository{Owner: u.Owner, Name: u.Repo})
		if err := ghscheduler.ApplyUpdate(client, u); err != nil {
			logrus.Warnf("Failed to update issue #%d: %v", u.IssueNum, err)
		} else {
			fmt.Printf("  Updated %s #%d\n", u.RepoKey, u.IssueNum)
		}
	}

	// Post or update scheduling issue comments
	if len(schedIssues) > 0 {
		fmt.Println("\nUpdating scheduling comments...")
		for _, si := range schedIssues {
			client := github.NewClient(accessToken, &github.GitHubRepository{Owner: si.Owner, Name: si.Repo})
			if err := ghscheduler.PostOrUpdateSchedulingComment(client, si); err != nil {
				logrus.Warnf("Failed to post comment for #%d: %v", si.IssueNum, err)
			} else {
				fmt.Printf("  Posted comment on %s/%s #%d\n", si.Owner, si.Repo, si.IssueNum)
			}
		}
	}

	// Delete comments for issues that are now schedulable
	fmt.Println("\nCleaning up resolved scheduling comments...")
	for ref, iwp := range allIssues {
		// Skip draft issues
		if iwp.IsDraft {
			continue
		}
		// Skip issues that still have problems
		if unschedulableIssues[ref] {
			continue
		}
		// Skip on-hold and closed issues (they don't need scheduling comments cleaned up)
		if iwp.SchedulingStatus == "On Hold" || strings.EqualFold(iwp.State, "closed") {
			continue
		}

		client := github.NewClient(accessToken, &github.GitHubRepository{Owner: iwp.Owner, Name: iwp.Repo})
		if err := ghscheduler.DeleteSchedulingComment(client, iwp.IssueNum); err != nil {
			logrus.Warnf("Failed to delete comment for #%d: %v", iwp.IssueNum, err)
		}
	}

	fmt.Println("Done!")
	return nil
}

func runDeviceFlow() (*github.StoredAuth, error) {
	config := github.GetDefaultConfig()

	fmt.Println("\nGitHub OAuth Authentication (Device Flow)")
	fmt.Println("==========================================")

	// Request device code
	deviceResp, err := github.RequestDeviceCode(config)
	if err != nil {
		return nil, fmt.Errorf("failed to request device code: %w", err)
	}

	// Display instructions
	fmt.Printf("\nTo complete authentication, visit:\n%s\n", deviceResp.VerificationURI)
	fmt.Printf("\nAnd enter code: %s\n", deviceResp.UserCode)
	fmt.Println("\nWaiting for you to complete authentication...")

	// Poll for token
	timeout := time.Duration(deviceResp.ExpiresIn) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	interval := time.Duration(deviceResp.Interval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("authentication timeout - device code expired")
		case <-ticker.C:
			tokenResp, err := github.PollForDeviceToken(config, deviceResp.DeviceCode, deviceResp.Interval)
			if err != nil {
				if err.Error() == "authorization_pending" {
					continue
				}
				if err.Error() == "slow_down" {
					ticker.Reset(interval * 2)
					continue
				}
				return nil, fmt.Errorf("device flow error: %w", err)
			}

			// Success! Verify and save
			username, err := github.GetAuthenticatedUser(tokenResp.AccessToken)
			if err != nil {
				return nil, fmt.Errorf("failed to verify token: %w", err)
			}

			// Save authentication (includes refresh token and expiration if provided)
			auth := github.TokenResponseToStoredAuth(tokenResp)

			if err := github.SaveStoredAuth(auth); err != nil {
				return nil, fmt.Errorf("failed to save auth: %w", err)
			}

			fmt.Printf("\nAuthenticated as: %s\n", username)
			if auth.RefreshToken != "" {
				fmt.Println("✓ Refresh token received - session will auto-renew")
			}
			return &auth, nil
		}
	}
}
