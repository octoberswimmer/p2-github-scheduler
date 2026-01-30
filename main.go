package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/octoberswimmer/p2/github"
	"github.com/octoberswimmer/p2/planner"
	"github.com/octoberswimmer/p2/planner/lseq"
	"github.com/octoberswimmer/p2/recfile"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	debug   bool
	dryRun  bool
	rootCmd = &cobra.Command{
		Use:   "p2-github-scheduler <github-url>",
		Short: "Reschedule GitHub issues using P2's scheduling algorithm",
		Long: `Fetches GitHub issues from a repository or project, runs P2's
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
		// Fall back to stored auth or device flow
		auth, err := github.LoadStoredAuth()
		if err != nil {
			fmt.Println("No stored GitHub authentication found. Starting device flow...")
			auth, err = runDeviceFlow()
			if err != nil {
				return fmt.Errorf("authentication failed: %w", err)
			}
		}

		// Verify token
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
	urlInfo, err := parseGitHubURL(url)
	if err != nil {
		return fmt.Errorf("invalid GitHub URL: %w", err)
	}

	var allIssues map[string]issueWithProject

	if urlInfo.isProject {
		// Fetch issues directly from the project
		fmt.Printf("Fetching items from project %s #%d...\n", urlInfo.owner, urlInfo.projectNum)
		allIssues, err = fetchProjectItems(accessToken, urlInfo)
		if err != nil {
			return err
		}
	} else {
		// Fetch issues from the repository
		fmt.Printf("Scheduling tasks for %s/%s\n", urlInfo.owner, urlInfo.repo)
		allIssues, err = fetchRepoIssues(accessToken, urlInfo)
		if err != nil {
			return err
		}
	}

	if len(allIssues) == 0 {
		fmt.Println("No issues to schedule")
		return nil
	}

	// Convert issues to P2 tasks
	fmt.Println("Converting to P2 tasks...")
	tasks, users := issuesToTasks(allIssues)
	fmt.Printf("Created %d tasks with %d users\n", len(tasks), len(users))

	if len(tasks) == 0 {
		fmt.Println("No tasks to schedule")
		return nil
	}

	// Run the scheduler
	fmt.Println("Running scheduler...")
	base := time.Now()
	entries := planner.ScheduleWithUsers(tasks, users)

	ganttData, err := planner.ComputeGanttData(entries, tasks, true, base, users)
	if err != nil {
		return fmt.Errorf("scheduling failed: %w", err)
	}

	// Prepare updates
	updates := prepareUpdates(ganttData, allIssues)

	if len(updates) == 0 {
		fmt.Println("No date changes needed")
		return nil
	}

	fmt.Printf("\nFound %d tasks with date changes:\n", len(updates))
	for _, u := range updates {
		fmt.Printf("  %s #%d %s\n", u.repoKey, u.issueNum, u.name)
		if u.clearDates {
			fmt.Println("       (clearing dates - task is closed or on hold)")
		} else {
			if !u.expectedStart.IsZero() {
				fmt.Printf("       Expected Start: %s\n", u.expectedStart.Format("2006-01-02"))
			}
			if !u.expectedCompletion.IsZero() {
				fmt.Printf("       Expected Completion: %s\n", u.expectedCompletion.Format("2006-01-02"))
			}
			if !u.completion98.IsZero() {
				fmt.Printf("       98%% Completion: %s\n", u.completion98.Format("2006-01-02"))
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
		client := github.NewClient(accessToken, &github.GitHubRepository{Owner: u.owner, Name: u.repo})
		if err := applyUpdate(client, u); err != nil {
			logrus.Warnf("Failed to update issue #%d: %v", u.issueNum, err)
		} else {
			fmt.Printf("  Updated %s #%d\n", u.repoKey, u.issueNum)
		}
	}

	fmt.Println("Done!")
	return nil
}

type urlInfo struct {
	owner      string
	repo       string
	isOrg      bool
	isProject  bool
	projectNum int
}

type issueWithProject struct {
	owner              string
	repo               string
	issueNum           int
	title              string
	body               string
	state              string
	assignee           string
	labels             []string
	milestone          string
	project            *github.ProjectItemInfo
	lowEstimate        float64
	highEstimate       float64
	order              int
	schedulingStatus   string // "On Hold", etc. from Scheduling Status field
	hasSchedulingDates bool   // true if any scheduling date fields are set
}

type dateUpdate struct {
	owner              string
	repo               string
	repoKey            string
	issueNum           int
	name               string
	project            *github.ProjectItemInfo
	expectedStart      time.Time
	expectedCompletion time.Time
	completion98       time.Time
	clearDates         bool // true if dates should be cleared (on-hold tasks)
}

func parseGitHubURL(url string) (*urlInfo, error) {
	// Remove trailing slashes and protocol
	url = strings.TrimSuffix(url, "/")
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Org project URL: github.com/orgs/org/projects/N
	orgProjectRe := regexp.MustCompile(`^github\.com/orgs/([^/]+)/projects/(\d+)`)
	if matches := orgProjectRe.FindStringSubmatch(url); matches != nil {
		num, _ := strconv.Atoi(matches[2])
		return &urlInfo{
			owner:      matches[1],
			isOrg:      true,
			isProject:  true,
			projectNum: num,
		}, nil
	}

	// User project URL: github.com/users/user/projects/N
	userProjectRe := regexp.MustCompile(`^github\.com/users/([^/]+)/projects/(\d+)`)
	if matches := userProjectRe.FindStringSubmatch(url); matches != nil {
		num, _ := strconv.Atoi(matches[2])
		return &urlInfo{
			owner:      matches[1],
			isOrg:      false,
			isProject:  true,
			projectNum: num,
		}, nil
	}

	// Repo URL: github.com/owner/repo or github.com/owner/repo/issues/N
	repoRe := regexp.MustCompile(`^github\.com/([^/]+)/([^/]+)`)
	if matches := repoRe.FindStringSubmatch(url); matches != nil {
		return &urlInfo{
			owner: matches[1],
			repo:  matches[2],
		}, nil
	}

	// Short form: owner/repo
	shortRe := regexp.MustCompile(`^([^/]+)/([^/]+)$`)
	if matches := shortRe.FindStringSubmatch(url); matches != nil {
		return &urlInfo{
			owner: matches[1],
			repo:  matches[2],
		}, nil
	}

	return nil, fmt.Errorf("could not parse GitHub URL: %s", url)
}

func fetchProjectItems(accessToken string, info *urlInfo) (map[string]issueWithProject, error) {
	project := &github.GitHubProject{
		Owner:  info.owner,
		Number: info.projectNum,
		IsOrg:  info.isOrg,
	}

	gqlClient := github.NewGraphQLClient(accessToken)

	// Fetch project field IDs once (instead of per-issue)
	projectFields, err := gqlClient.GetProjectFields(project)
	if err != nil {
		logrus.Warnf("Failed to fetch project fields: %v", err)
		// Continue without field IDs - updates won't work but we can still schedule
	} else {
		logrus.Debugf("Project fields: %v", projectFields.FieldIDs)
	}

	items, err := gqlClient.GetProjectItems(project)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch project items: %w", err)
	}

	fmt.Printf("Found %d items in project\n", len(items))

	allIssues := make(map[string]issueWithProject)

	// Track position for ordering (GraphQL returns items in display order)
	position := 0
	for _, item := range items {
		// Skip non-issues (PRs, draft issues)
		if item.ContentType != "ISSUE" && item.IssueNumber == 0 {
			continue
		}

		ref := fmt.Sprintf("github.com/%s/%s/issues/%d", item.RepoOwner, item.RepoName, item.IssueNumber)

		// Extract estimates and status from custom fields
		var lowEstimate, highEstimate float64
		var schedulingStatus string
		var hasSchedulingFields bool
		if v, ok := item.CustomFields["Low Estimate"].(float64); ok {
			lowEstimate = v
			hasSchedulingFields = true
		}
		if v, ok := item.CustomFields["High Estimate"].(float64); ok {
			highEstimate = v
			hasSchedulingFields = true
		}
		if v, ok := item.CustomFields["Scheduling Status"].(string); ok {
			schedulingStatus = v
		}
		// Check if any scheduling date fields are set
		if _, ok := item.CustomFields["Expected Start"].(string); ok {
			hasSchedulingFields = true
		}
		if _, ok := item.CustomFields["Expected Completion"].(string); ok {
			hasSchedulingFields = true
		}
		if _, ok := item.CustomFields["98% Completion"].(string); ok {
			hasSchedulingFields = true
		}

		// Build project info for this item using the project-level field IDs
		var projectInfo *github.ProjectItemInfo
		if projectFields != nil {
			projectInfo = &github.ProjectItemInfo{
				ProjectID:           projectFields.ProjectID,
				ItemID:              item.ID,
				FieldIDs:            projectFields.FieldIDs,
				SingleSelectOptions: projectFields.SingleSelectOptions,
			}
		}

		var assignee string
		if len(item.Assignees) > 0 {
			assignee = item.Assignees[0]
		}

		allIssues[ref] = issueWithProject{
			owner:              item.RepoOwner,
			repo:               item.RepoName,
			issueNum:           item.IssueNumber,
			title:              item.Title,
			body:               item.Body,
			state:              item.State,
			assignee:           assignee,
			labels:             item.Labels,
			milestone:          item.Milestone,
			project:            projectInfo,
			lowEstimate:        lowEstimate,
			highEstimate:       highEstimate,
			order:              position,
			schedulingStatus:   schedulingStatus,
			hasSchedulingDates: hasSchedulingFields,
		}

		logrus.Debugf("Issue %s: low=%.1f, high=%.1f, order=%d, schedulingStatus=%s, hasFields=%v",
			ref, lowEstimate, highEstimate, position, schedulingStatus, hasSchedulingFields)
		position++
	}

	return allIssues, nil
}

func fetchRepoIssues(accessToken string, info *urlInfo) (map[string]issueWithProject, error) {
	client := github.NewClient(accessToken, &github.GitHubRepository{Owner: info.owner, Name: info.repo})

	fmt.Println("Fetching issues...")
	issues, err := client.ListIssues("all")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch issues: %w", err)
	}
	fmt.Printf("Found %d issues\n", len(issues))

	if len(issues) == 0 {
		return nil, nil
	}

	allIssues := make(map[string]issueWithProject)

	fmt.Println("Fetching project information for issues...")
	for _, issue := range issues {
		issueData, err := client.GetIssueWithCustomFields(issue.Number)
		if err != nil {
			logrus.Warnf("Failed to get custom fields for issue #%d: %v", issue.Number, err)
			continue
		}

		projectInfo := github.ExtractProjectInfo(issueData)
		if projectInfo == nil {
			logrus.Debugf("Issue #%d is not in any project", issue.Number)
			continue
		}

		lowEstimate, highEstimate, order := github.ExtractCustomFields(issueData)
		schedulingStatus := extractSchedulingStatus(issueData)

		ref := fmt.Sprintf("github.com/%s/%s/issues/%d", info.owner, info.repo, issue.Number)

		var assignee string
		if issue.Assignee != nil {
			assignee = issue.Assignee.Login
		}

		var labels []string
		for _, l := range issue.Labels {
			labels = append(labels, l.Name)
		}

		var milestone string
		if issue.Milestone != nil {
			milestone = issue.Milestone.Title
		}

		allIssues[ref] = issueWithProject{
			owner:            info.owner,
			repo:             info.repo,
			issueNum:         issue.Number,
			title:            issue.Title,
			body:             issue.Body,
			state:            issue.State,
			assignee:         assignee,
			labels:           labels,
			milestone:        milestone,
			project:          projectInfo,
			lowEstimate:      lowEstimate,
			highEstimate:     highEstimate,
			order:            order,
			schedulingStatus: schedulingStatus,
		}

		logrus.Debugf("Issue #%d is in project %s (low=%.1f, high=%.1f, order=%d, schedulingStatus=%s)",
			issue.Number, projectInfo.ProjectID, lowEstimate, highEstimate, order, schedulingStatus)
	}

	return allIssues, nil
}

// extractSchedulingStatus extracts the "Scheduling Status" single-select field value from issue data
func extractSchedulingStatus(issueData map[string]interface{}) string {
	repo, ok := issueData["repository"].(map[string]interface{})
	if !ok {
		return ""
	}

	issue, ok := repo["issue"].(map[string]interface{})
	if !ok {
		return ""
	}

	projectItems, ok := issue["projectItems"].(map[string]interface{})
	if !ok {
		return ""
	}

	nodes, ok := projectItems["nodes"].([]interface{})
	if !ok || len(nodes) == 0 {
		return ""
	}

	// Check first project item for Scheduling Status field
	for _, node := range nodes {
		projectItem, ok := node.(map[string]interface{})
		if !ok {
			continue
		}

		fieldValues, ok := projectItem["fieldValues"].(map[string]interface{})
		if !ok {
			continue
		}

		fieldNodes, ok := fieldValues["nodes"].([]interface{})
		if !ok {
			continue
		}

		for _, fieldNode := range fieldNodes {
			field, ok := fieldNode.(map[string]interface{})
			if !ok {
				continue
			}

			fieldInfo, ok := field["field"].(map[string]interface{})
			if !ok {
				continue
			}

			fieldName, _ := fieldInfo["name"].(string)

			// Extract single-select fields (have "name" property for the selected option)
			if fieldName == "Scheduling Status" {
				if name, ok := field["name"].(string); ok {
					return name
				}
			}
		}
	}

	return ""
}

func issuesToTasks(issues map[string]issueWithProject) ([]planner.Task, []recfile.User) {
	gen := lseq.NewGenerator("scheduler")
	userSet := make(map[string]bool)
	var tasks []planner.Task

	// Convert map to slice and sort by order to preserve GitHub Project ordering
	type refIssue struct {
		ref string
		iwp issueWithProject
	}
	sortedIssues := make([]refIssue, 0, len(issues))
	for ref, iwp := range issues {
		sortedIssues = append(sortedIssues, refIssue{ref: ref, iwp: iwp})
	}
	sort.Slice(sortedIssues, func(i, j int) bool {
		return sortedIssues[i].iwp.order < sortedIssues[j].iwp.order
	})

	for i, ri := range sortedIssues {
		ref := ri.ref
		iwp := ri.iwp
		task := planner.Task{
			ID:           fmt.Sprintf("%s/%s#%d", iwp.owner, iwp.repo, iwp.issueNum),
			Sequence:     lseq.SequentialString(i, "scheduler"),
			Name:         iwp.title,
			Ref:          []string{ref},
			Done:         strings.EqualFold(iwp.state, "closed"),
			EstimateLow:  iwp.lowEstimate,
			EstimateHigh: iwp.highEstimate,
		}

		// Default estimates if not set
		if task.EstimateLow == 0 && task.EstimateHigh == 0 && !task.Done {
			task.EstimateLow = 1
			task.EstimateHigh = 4
		}

		// Extract assignee
		if iwp.assignee != "" {
			task.User = iwp.assignee
			userSet[iwp.assignee] = true
		}

		// Check for on-hold via Scheduling Status field
		if iwp.schedulingStatus == "On Hold" {
			task.OnHold = true
		}

		// Extract milestone as package
		if iwp.milestone != "" {
			task.PackageID = iwp.milestone
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

	return tasks, users
}

func prepareUpdates(ganttData planner.GanttData, issues map[string]issueWithProject) []dateUpdate {
	var updates []dateUpdate

	// Build a map from task ID to issue
	taskToIssue := make(map[string]issueWithProject)
	for ref, iwp := range issues {
		taskID := fmt.Sprintf("%s/%s#%d", iwp.owner, iwp.repo, iwp.issueNum)
		taskToIssue[taskID] = iwp
		_ = ref // ref is stored in the issue
	}

	for _, bar := range ganttData.Bars {
		if bar.IsPackage {
			continue
		}

		iwp, ok := taskToIssue[bar.ID]
		if !ok {
			logrus.Debugf("No issue found for task %s", bar.ID)
			continue
		}

		if iwp.project == nil {
			logrus.Debugf("Issue %s is not in a project", bar.ID)
			continue
		}

		// Closed and on-hold tasks should have their dates cleared (only if they have dates set)
		if bar.Done || bar.OnHold {
			if !iwp.hasSchedulingDates {
				continue
			}
			update := dateUpdate{
				owner:      iwp.owner,
				repo:       iwp.repo,
				repoKey:    fmt.Sprintf("%s/%s", iwp.owner, iwp.repo),
				issueNum:   iwp.issueNum,
				name:       bar.Name,
				project:    iwp.project,
				clearDates: true,
			}
			updates = append(updates, update)
			continue
		}

		update := dateUpdate{
			owner:              iwp.owner,
			repo:               iwp.repo,
			repoKey:            fmt.Sprintf("%s/%s", iwp.owner, iwp.repo),
			issueNum:           iwp.issueNum,
			name:               bar.Name,
			project:            iwp.project,
			expectedStart:      bar.ExpStartDate,
			expectedCompletion: bar.MeanDate,
			completion98:       bar.End98Date,
		}

		updates = append(updates, update)
	}

	return updates
}

func applyUpdate(client *github.Client, update dateUpdate) error {
	if update.project == nil {
		return fmt.Errorf("no project info")
	}

	// Clear all scheduling fields for closed/on-hold tasks
	if update.clearDates {
		fieldsToClean := []string{
			"Expected Start",
			"Expected Completion",
			"98% Completion",
			"Low Estimate",
			"High Estimate",
		}
		for _, fieldName := range fieldsToClean {
			if fieldID, ok := update.project.FieldIDs[fieldName]; ok {
				if err := client.ClearField(update.project.ProjectID, update.project.ItemID, fieldID); err != nil {
					logrus.Warnf("Failed to clear %s for #%d: %v", fieldName, update.issueNum, err)
				}
			}
		}
		return nil
	}

	// Update Expected Start
	if !update.expectedStart.IsZero() {
		if fieldID, ok := update.project.FieldIDs["Expected Start"]; ok {
			if err := client.UpdateDateField(update.project.ProjectID, update.project.ItemID, fieldID, update.expectedStart); err != nil {
				logrus.Warnf("Failed to update Expected Start for #%d: %v", update.issueNum, err)
			}
		} else {
			logrus.Debugf("No 'Expected Start' field found for issue #%d", update.issueNum)
		}
	}

	// Update Expected Completion
	if !update.expectedCompletion.IsZero() {
		if fieldID, ok := update.project.FieldIDs["Expected Completion"]; ok {
			if err := client.UpdateDateField(update.project.ProjectID, update.project.ItemID, fieldID, update.expectedCompletion); err != nil {
				logrus.Warnf("Failed to update Expected Completion for #%d: %v", update.issueNum, err)
			}
		} else {
			logrus.Debugf("No 'Expected Completion' field found for issue #%d", update.issueNum)
		}
	}

	// Update 98% Completion
	if !update.completion98.IsZero() {
		if fieldID, ok := update.project.FieldIDs["98% Completion"]; ok {
			if err := client.UpdateDateField(update.project.ProjectID, update.project.ItemID, fieldID, update.completion98); err != nil {
				logrus.Warnf("Failed to update 98%% Completion for #%d: %v", update.issueNum, err)
			}
		} else {
			logrus.Debugf("No '98%% Completion' field found for issue #%d", update.issueNum)
		}
	}

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

			auth := github.StoredAuth{
				AccessToken: tokenResp.AccessToken,
				TokenType:   tokenResp.TokenType,
				Scope:       tokenResp.Scope,
			}

			if err := github.SaveStoredAuth(auth); err != nil {
				return nil, fmt.Errorf("failed to save auth: %w", err)
			}

			fmt.Printf("\nAuthenticated as: %s\n", username)
			return &auth, nil
		}
	}
}
