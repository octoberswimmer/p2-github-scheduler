package main

import (
	"testing"

	"github.com/octoberswimmer/p2/github"
	"github.com/octoberswimmer/p2/planner"
)

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantOwner  string
		wantRepo   string
		wantIsOrg  bool
		wantIsPrj  bool
		wantPrjNum int
		wantErr    bool
	}{
		{
			name:      "full_repo_url",
			url:       "https://github.com/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "repo_url_with_trailing_slash",
			url:       "https://github.com/owner/repo/",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "issue_url",
			url:       "https://github.com/owner/repo/issues/123",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "short_form",
			url:       "owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:       "org_project_url",
			url:        "https://github.com/orgs/myorg/projects/1",
			wantOwner:  "myorg",
			wantIsOrg:  true,
			wantIsPrj:  true,
			wantPrjNum: 1,
		},
		{
			name:       "user_project_url",
			url:        "https://github.com/users/myuser/projects/42",
			wantOwner:  "myuser",
			wantIsOrg:  false,
			wantIsPrj:  true,
			wantPrjNum: 42,
		},
		{
			name:      "owner_with_dashes",
			url:       "https://github.com/my-org/my-repo",
			wantOwner: "my-org",
			wantRepo:  "my-repo",
		},
		{
			name:    "invalid_url",
			url:     "not-a-github-url",
			wantErr: true,
		},
		{
			name:    "empty_url",
			url:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := parseGitHubURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseGitHubURL(%q) expected error, got nil", tt.url)
				}
				return
			}
			if err != nil {
				t.Errorf("parseGitHubURL(%q) unexpected error: %v", tt.url, err)
				return
			}
			if info.owner != tt.wantOwner {
				t.Errorf("parseGitHubURL(%q) owner = %q, want %q", tt.url, info.owner, tt.wantOwner)
			}
			if info.repo != tt.wantRepo {
				t.Errorf("parseGitHubURL(%q) repo = %q, want %q", tt.url, info.repo, tt.wantRepo)
			}
			if info.isOrg != tt.wantIsOrg {
				t.Errorf("parseGitHubURL(%q) isOrg = %v, want %v", tt.url, info.isOrg, tt.wantIsOrg)
			}
			if info.isProject != tt.wantIsPrj {
				t.Errorf("parseGitHubURL(%q) isProject = %v, want %v", tt.url, info.isProject, tt.wantIsPrj)
			}
			if info.projectNum != tt.wantPrjNum {
				t.Errorf("parseGitHubURL(%q) projectNum = %d, want %d", tt.url, info.projectNum, tt.wantPrjNum)
			}
		})
	}
}

func TestExtractSchedulingStatus(t *testing.T) {
	tests := []struct {
		name       string
		issueData  map[string]interface{}
		wantStatus string
	}{
		{
			name:       "empty_data",
			issueData:  map[string]interface{}{},
			wantStatus: "",
		},
		{
			name: "on_hold_status",
			issueData: map[string]interface{}{
				"repository": map[string]interface{}{
					"issue": map[string]interface{}{
						"projectItems": map[string]interface{}{
							"nodes": []interface{}{
								map[string]interface{}{
									"fieldValues": map[string]interface{}{
										"nodes": []interface{}{
											map[string]interface{}{
												"field": map[string]interface{}{
													"name": "Scheduling Status",
												},
												"name": "On Hold",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantStatus: "On Hold",
		},
		{
			name: "active_status",
			issueData: map[string]interface{}{
				"repository": map[string]interface{}{
					"issue": map[string]interface{}{
						"projectItems": map[string]interface{}{
							"nodes": []interface{}{
								map[string]interface{}{
									"fieldValues": map[string]interface{}{
										"nodes": []interface{}{
											map[string]interface{}{
												"field": map[string]interface{}{
													"name": "Scheduling Status",
												},
												"name": "Active",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantStatus: "Active",
		},
		{
			name: "no_scheduling_status_field",
			issueData: map[string]interface{}{
				"repository": map[string]interface{}{
					"issue": map[string]interface{}{
						"projectItems": map[string]interface{}{
							"nodes": []interface{}{
								map[string]interface{}{
									"fieldValues": map[string]interface{}{
										"nodes": []interface{}{
											map[string]interface{}{
												"field": map[string]interface{}{
													"name": "Other Field",
												},
												"name": "Some Value",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantStatus: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSchedulingStatus(tt.issueData)
			if got != tt.wantStatus {
				t.Errorf("extractSchedulingStatus() = %q, want %q", got, tt.wantStatus)
			}
		})
	}
}

func TestIssuesToTasks_OnHoldViaSchedulingStatus(t *testing.T) {
	issues := map[string]issueWithProject{
		"github.com/owner/repo/issues/1": {
			owner:            "owner",
			repo:             "repo",
			issueNum:         1,
			title:            "On Hold Task",
			state:            "open",
			schedulingStatus: "On Hold",
		},
		"github.com/owner/repo/issues/2": {
			owner:            "owner",
			repo:             "repo",
			issueNum:         2,
			title:            "Active Task",
			state:            "open",
			schedulingStatus: "Active",
		},
		"github.com/owner/repo/issues/3": {
			owner:    "owner",
			repo:     "repo",
			issueNum: 3,
			title:    "No Status Task",
			state:    "open",
		},
	}

	tasks, _ := issuesToTasks(issues)

	// Find tasks by title
	taskMap := make(map[string]planner.Task)
	for _, t := range tasks {
		taskMap[t.Name] = t
	}

	if !taskMap["On Hold Task"].OnHold {
		t.Error("expected 'On Hold Task' to have OnHold=true")
	}
	if taskMap["Active Task"].OnHold {
		t.Error("expected 'Active Task' to have OnHold=false")
	}
	if taskMap["No Status Task"].OnHold {
		t.Error("expected 'No Status Task' to have OnHold=false")
	}
}

func TestIssuesToTasks_DraftIssuesAreOnHold(t *testing.T) {
	issues := map[string]issueWithProject{
		"github.com/owner/repo/issues/1": {
			owner:    "owner",
			repo:     "repo",
			issueNum: 1,
			title:    "Regular Issue",
			state:    "open",
		},
		"draft:PVTI_abc123": {
			title:         "Draft Issue",
			isDraft:       true,
			projectItemID: "PVTI_abc123",
		},
	}

	tasks, _ := issuesToTasks(issues)

	// Find tasks by title
	taskMap := make(map[string]planner.Task)
	for _, t := range tasks {
		taskMap[t.Name] = t
	}

	if taskMap["Regular Issue"].OnHold {
		t.Error("expected 'Regular Issue' to have OnHold=false")
	}
	if !taskMap["Draft Issue"].OnHold {
		t.Error("expected 'Draft Issue' to have OnHold=true")
	}
	if taskMap["Draft Issue"].ID != "draft:PVTI_abc123" {
		t.Errorf("expected draft issue ID 'draft:PVTI_abc123', got %q", taskMap["Draft Issue"].ID)
	}
}

func TestPrepareUpdates_OnHoldTasksGetCleared(t *testing.T) {
	// Create a mock project info
	projectInfo := &github.ProjectItemInfo{
		ProjectID: "proj-1",
		ItemID:    "item-1",
		FieldIDs: map[string]string{
			"Expected Start":      "field-1",
			"Expected Completion": "field-2",
			"98% Completion":      "field-3",
		},
	}

	// Create mock issues - one on-hold with dates, one active
	issues := map[string]issueWithProject{
		"github.com/owner/repo/issues/1": {
			owner:              "owner",
			repo:               "repo",
			issueNum:           1,
			title:              "On Hold Task",
			state:              "open",
			schedulingStatus:   "On Hold",
			project:            projectInfo,
			hasSchedulingDates: true,
		},
		"github.com/owner/repo/issues/2": {
			owner:    "owner",
			repo:     "repo",
			issueNum: 2,
			title:    "Active Task",
			state:    "open",
			project:  projectInfo,
		},
	}

	// Create mock gantt data with both tasks
	ganttData := planner.GanttData{
		Bars: []planner.GanttBar{
			{
				ID:     "owner/repo#1",
				Name:   "On Hold Task",
				OnHold: true,
			},
			{
				ID:   "owner/repo#2",
				Name: "Active Task",
			},
		},
	}

	updates := prepareUpdates(ganttData, issues)

	// Should have 2 updates
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}

	// Find the on-hold task update
	var onHoldUpdate, activeUpdate *dateUpdate
	for i := range updates {
		if updates[i].issueNum == 1 {
			onHoldUpdate = &updates[i]
		} else if updates[i].issueNum == 2 {
			activeUpdate = &updates[i]
		}
	}

	if onHoldUpdate == nil {
		t.Fatal("expected to find on-hold task update")
	}
	if !onHoldUpdate.clearDates {
		t.Error("expected on-hold task to have clearDates=true")
	}
	if onHoldUpdate.clearReason != "on hold" {
		t.Errorf("expected on-hold task clearReason=%q, got %q", "on hold", onHoldUpdate.clearReason)
	}

	if activeUpdate == nil {
		t.Fatal("expected to find active task update")
	}
	if activeUpdate.clearDates {
		t.Error("expected active task to have clearDates=false")
	}
}

func TestPrepareUpdates_OnHoldWithOnlyEstimatesSkipped(t *testing.T) {
	// On-hold tasks should only be included if they have dates set
	// Estimates alone should not trigger clearing (we keep estimates for on-hold)
	projectInfo := &github.ProjectItemInfo{
		ProjectID: "proj-1",
		ItemID:    "item-1",
		FieldIDs: map[string]string{
			"Expected Start":      "field-1",
			"Expected Completion": "field-2",
			"98% Completion":      "field-3",
			"Low Estimate":        "field-4",
			"High Estimate":       "field-5",
		},
	}

	issues := map[string]issueWithProject{
		"github.com/owner/repo/issues/1": {
			owner:            "owner",
			repo:             "repo",
			issueNum:         1,
			title:            "On Hold With Only Estimates",
			state:            "open",
			schedulingStatus: "On Hold",
			project:          projectInfo,
			lowEstimate:      2,
			highEstimate:     8,
			hasEstimates:     true,
			// hasSchedulingDates is false - no dates set
		},
	}

	ganttData := planner.GanttData{
		Bars: []planner.GanttBar{
			{
				ID:     "owner/repo#1",
				Name:   "On Hold With Only Estimates",
				OnHold: true,
			},
		},
	}

	updates := prepareUpdates(ganttData, issues)

	// Should have 0 updates - on-hold with only estimates should be skipped
	if len(updates) != 0 {
		t.Fatalf("expected 0 updates for on-hold task with only estimates, got %d", len(updates))
	}
}

func TestIssuesToTasks_PreservesOrder(t *testing.T) {
	// Create issues with specific order values (simulating GitHub Project order)
	issues := map[string]issueWithProject{
		"github.com/owner/repo/issues/5": {
			owner:    "owner",
			repo:     "repo",
			issueNum: 5,
			title:    "Third Task",
			state:    "open",
			order:    2,
		},
		"github.com/owner/repo/issues/1": {
			owner:    "owner",
			repo:     "repo",
			issueNum: 1,
			title:    "First Task",
			state:    "open",
			order:    0,
		},
		"github.com/owner/repo/issues/10": {
			owner:    "owner",
			repo:     "repo",
			issueNum: 10,
			title:    "Second Task",
			state:    "open",
			order:    1,
		},
	}

	tasks, _ := issuesToTasks(issues)

	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	// Tasks should be sorted by order, not by issue number or map iteration order
	expectedOrder := []string{"First Task", "Second Task", "Third Task"}
	for i, task := range tasks {
		if task.Name != expectedOrder[i] {
			t.Errorf("task %d: expected name %q, got %q", i, expectedOrder[i], task.Name)
		}
	}
}

func TestIssuesToTasks_SkipsInaccessibleDependencies(t *testing.T) {
	// Task 2 depends on task 1 (accessible) and task 3 (inaccessible - not in issues map)
	issues := map[string]issueWithProject{
		"github.com/owner/repo/issues/1": {
			owner:    "owner",
			repo:     "repo",
			issueNum: 1,
			title:    "Blocker Task",
			state:    "open",
			order:    0,
		},
		"github.com/owner/repo/issues/2": {
			owner:    "owner",
			repo:     "repo",
			issueNum: 2,
			title:    "Blocked Task",
			state:    "open",
			order:    1,
			blockedBy: []github.IssueRef{
				{Owner: "owner", Repo: "repo", Number: 1},     // accessible
				{Owner: "other", Repo: "private", Number: 99}, // inaccessible
			},
		},
	}

	tasks, _ := issuesToTasks(issues)

	// Find the blocked task
	var blockedTask *planner.Task
	for i := range tasks {
		if tasks[i].Name == "Blocked Task" {
			blockedTask = &tasks[i]
			break
		}
	}

	if blockedTask == nil {
		t.Fatal("Blocked Task not found")
	}

	// Should only have one dependency (the accessible one)
	if len(blockedTask.DependsOn) != 1 {
		t.Errorf("expected 1 dependency, got %d: %v", len(blockedTask.DependsOn), blockedTask.DependsOn)
	}

	if len(blockedTask.DependsOn) > 0 && blockedTask.DependsOn[0] != "owner/repo#1" {
		t.Errorf("expected dependency on 'owner/repo#1', got %q", blockedTask.DependsOn[0])
	}
}

func TestIssuesToTasks_UnassignedTasksGetDefaultUser(t *testing.T) {
	issues := map[string]issueWithProject{
		"github.com/owner/repo/issues/1": {
			owner:    "owner",
			repo:     "repo",
			issueNum: 1,
			title:    "Assigned Task",
			state:    "open",
			assignee: "cwarden",
		},
		"github.com/owner/repo/issues/2": {
			owner:    "owner",
			repo:     "repo",
			issueNum: 2,
			title:    "Unassigned Task",
			state:    "open",
			// no assignee
		},
	}

	tasks, users := issuesToTasks(issues)

	// Find tasks by title
	taskMap := make(map[string]planner.Task)
	for _, task := range tasks {
		taskMap[task.Name] = task
	}

	// Assigned task should have the assignee
	if taskMap["Assigned Task"].User != "cwarden" {
		t.Errorf("expected assigned task to have User='cwarden', got %q", taskMap["Assigned Task"].User)
	}

	// Unassigned task should have "unassigned" user
	if taskMap["Unassigned Task"].User != "unassigned" {
		t.Errorf("expected unassigned task to have User='unassigned', got %q", taskMap["Unassigned Task"].User)
	}

	// Users slice should include both cwarden and unassigned
	userMap := make(map[string]bool)
	for _, u := range users {
		userMap[u.ID] = true
	}
	if !userMap["cwarden"] {
		t.Error("expected users to include 'cwarden'")
	}
	if !userMap["unassigned"] {
		t.Error("expected users to include 'unassigned'")
	}
}

func TestBuildReverseDependencies_BlockingCreatesBlockedBy(t *testing.T) {
	// Issue A blocks issue B - B should get A added to its blockedBy
	issues := map[string]issueWithProject{
		"github.com/owner/repo/issues/1": {
			owner:    "owner",
			repo:     "repo",
			issueNum: 1,
			title:    "Blocker Task",
			state:    "open",
			blocking: []github.IssueRef{
				{Owner: "owner", Repo: "repo", Number: 2}, // blocks issue 2
			},
		},
		"github.com/owner/repo/issues/2": {
			owner:    "owner",
			repo:     "repo",
			issueNum: 2,
			title:    "Blocked Task",
			state:    "open",
			// blockedBy is empty initially
		},
	}

	buildReverseDependencies(issues)

	blockedIssue := issues["github.com/owner/repo/issues/2"]
	if len(blockedIssue.blockedBy) != 1 {
		t.Errorf("expected 1 blockedBy entry, got %d", len(blockedIssue.blockedBy))
	}

	if len(blockedIssue.blockedBy) > 0 {
		ref := blockedIssue.blockedBy[0]
		if ref.Owner != "owner" || ref.Repo != "repo" || ref.Number != 1 {
			t.Errorf("expected blockedBy to reference owner/repo#1, got %s/%s#%d", ref.Owner, ref.Repo, ref.Number)
		}
	}
}

func TestBuildReverseDependencies_CrossRepoBlocking(t *testing.T) {
	// Issue in repo A blocks issue in repo B
	issues := map[string]issueWithProject{
		"github.com/owner/repoA/issues/1": {
			owner:    "owner",
			repo:     "repoA",
			issueNum: 1,
			title:    "Blocker in Repo A",
			state:    "open",
			blocking: []github.IssueRef{
				{Owner: "owner", Repo: "repoB", Number: 1}, // blocks issue in repo B
			},
		},
		"github.com/owner/repoB/issues/1": {
			owner:    "owner",
			repo:     "repoB",
			issueNum: 1,
			title:    "Blocked in Repo B",
			state:    "open",
		},
	}

	buildReverseDependencies(issues)

	blockedIssue := issues["github.com/owner/repoB/issues/1"]
	if len(blockedIssue.blockedBy) != 1 {
		t.Errorf("expected 1 blockedBy entry for cross-repo dependency, got %d", len(blockedIssue.blockedBy))
	}

	if len(blockedIssue.blockedBy) > 0 {
		ref := blockedIssue.blockedBy[0]
		if ref.Owner != "owner" || ref.Repo != "repoA" || ref.Number != 1 {
			t.Errorf("expected blockedBy to reference owner/repoA#1, got %s/%s#%d", ref.Owner, ref.Repo, ref.Number)
		}
	}
}

func TestBuildReverseDependencies_NoDuplicates(t *testing.T) {
	// Issue already has blockedBy set - shouldn't add duplicate
	issues := map[string]issueWithProject{
		"github.com/owner/repo/issues/1": {
			owner:    "owner",
			repo:     "repo",
			issueNum: 1,
			title:    "Blocker Task",
			state:    "open",
			blocking: []github.IssueRef{
				{Owner: "owner", Repo: "repo", Number: 2},
			},
		},
		"github.com/owner/repo/issues/2": {
			owner:    "owner",
			repo:     "repo",
			issueNum: 2,
			title:    "Blocked Task",
			state:    "open",
			blockedBy: []github.IssueRef{
				{Owner: "owner", Repo: "repo", Number: 1}, // already has the dependency
			},
		},
	}

	buildReverseDependencies(issues)

	blockedIssue := issues["github.com/owner/repo/issues/2"]
	if len(blockedIssue.blockedBy) != 1 {
		t.Errorf("expected 1 blockedBy entry (no duplicate), got %d", len(blockedIssue.blockedBy))
	}
}

func TestPrepareUpdates_ClosedTasksGetCleared(t *testing.T) {
	// Create a mock project info
	projectInfo := &github.ProjectItemInfo{
		ProjectID: "proj-1",
		ItemID:    "item-1",
		FieldIDs: map[string]string{
			"Expected Start":      "field-1",
			"Expected Completion": "field-2",
			"98% Completion":      "field-3",
		},
	}

	// Create mock issues - one closed with dates, one open
	issues := map[string]issueWithProject{
		"github.com/owner/repo/issues/1": {
			owner:              "owner",
			repo:               "repo",
			issueNum:           1,
			title:              "Closed Task",
			state:              "closed",
			project:            projectInfo,
			hasSchedulingDates: true,
		},
		"github.com/owner/repo/issues/2": {
			owner:    "owner",
			repo:     "repo",
			issueNum: 2,
			title:    "Open Task",
			state:    "open",
			project:  projectInfo,
		},
	}

	// Create mock gantt data with both tasks
	ganttData := planner.GanttData{
		Bars: []planner.GanttBar{
			{
				ID:   "owner/repo#1",
				Name: "Closed Task",
				Done: true,
			},
			{
				ID:   "owner/repo#2",
				Name: "Open Task",
			},
		},
	}

	updates := prepareUpdates(ganttData, issues)

	// Should have 2 updates
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}

	// Find the closed task update
	var closedUpdate, openUpdate *dateUpdate
	for i := range updates {
		if updates[i].issueNum == 1 {
			closedUpdate = &updates[i]
		} else if updates[i].issueNum == 2 {
			openUpdate = &updates[i]
		}
	}

	if closedUpdate == nil {
		t.Fatal("expected to find closed task update")
	}
	if !closedUpdate.clearDates {
		t.Error("expected closed task to have clearDates=true")
	}
	if closedUpdate.clearReason != "closed" {
		t.Errorf("expected closed task clearReason=%q, got %q", "closed", closedUpdate.clearReason)
	}

	if openUpdate == nil {
		t.Fatal("expected to find open task update")
	}
	if openUpdate.clearDates {
		t.Error("expected open task to have clearDates=false")
	}
}

func TestPrepareUpdates_ClosedTasksWithoutDatesSkipped(t *testing.T) {
	// Create a mock project info
	projectInfo := &github.ProjectItemInfo{
		ProjectID: "proj-1",
		ItemID:    "item-1",
		FieldIDs: map[string]string{
			"Expected Start":      "field-1",
			"Expected Completion": "field-2",
			"98% Completion":      "field-3",
		},
	}

	// Create mock issues - one closed without dates, one open
	issues := map[string]issueWithProject{
		"github.com/owner/repo/issues/1": {
			owner:              "owner",
			repo:               "repo",
			issueNum:           1,
			title:              "Closed Task Without Dates",
			state:              "closed",
			project:            projectInfo,
			hasSchedulingDates: false, // no dates to clear
		},
		"github.com/owner/repo/issues/2": {
			owner:    "owner",
			repo:     "repo",
			issueNum: 2,
			title:    "Open Task",
			state:    "open",
			project:  projectInfo,
		},
	}

	// Create mock gantt data with both tasks
	ganttData := planner.GanttData{
		Bars: []planner.GanttBar{
			{
				ID:   "owner/repo#1",
				Name: "Closed Task Without Dates",
				Done: true,
			},
			{
				ID:   "owner/repo#2",
				Name: "Open Task",
			},
		},
	}

	updates := prepareUpdates(ganttData, issues)

	// Should have only 1 update (the open task), closed task without dates is skipped
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}

	if updates[0].issueNum != 2 {
		t.Errorf("expected open task update, got issue #%d", updates[0].issueNum)
	}
}

func TestPrepareUpdates_ClosedTasksWithEstimatesGetCleared(t *testing.T) {
	// Create a mock project info with all scheduling fields
	projectInfo := &github.ProjectItemInfo{
		ProjectID: "proj-1",
		ItemID:    "item-1",
		FieldIDs: map[string]string{
			"Expected Start":      "field-1",
			"Expected Completion": "field-2",
			"98% Completion":      "field-3",
			"Low Estimate":        "field-4",
			"High Estimate":       "field-5",
		},
	}

	// Create mock issues - one closed with estimates (no dates), one open
	issues := map[string]issueWithProject{
		"github.com/owner/repo/issues/1": {
			owner:        "owner",
			repo:         "repo",
			issueNum:     1,
			title:        "Closed Task With Estimates",
			state:        "closed",
			project:      projectInfo,
			lowEstimate:  2,
			highEstimate: 8,
			hasEstimates: true, // has estimates to clear (no dates)
		},
		"github.com/owner/repo/issues/2": {
			owner:    "owner",
			repo:     "repo",
			issueNum: 2,
			title:    "Open Task",
			state:    "open",
			project:  projectInfo,
		},
	}

	// Create mock gantt data with both tasks
	ganttData := planner.GanttData{
		Bars: []planner.GanttBar{
			{
				ID:   "owner/repo#1",
				Name: "Closed Task With Estimates",
				Done: true,
			},
			{
				ID:   "owner/repo#2",
				Name: "Open Task",
			},
		},
	}

	updates := prepareUpdates(ganttData, issues)

	// Should have 2 updates - closed task with estimates should be included
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}

	// Find the closed task update
	var closedUpdate *dateUpdate
	for i := range updates {
		if updates[i].issueNum == 1 {
			closedUpdate = &updates[i]
			break
		}
	}

	if closedUpdate == nil {
		t.Fatal("expected to find closed task update")
	}
	if !closedUpdate.clearDates {
		t.Error("expected closed task with estimates to have clearDates=true")
	}
	if closedUpdate.clearReason != "closed" {
		t.Errorf("expected closed task clearReason=%q, got %q", "closed", closedUpdate.clearReason)
	}
}

func TestPrepareUpdates_OnHoldDetectedFromGitHubNotGantt(t *testing.T) {
	// On-hold status should be detected from GitHub's Scheduling Status field,
	// not from the GanttBar. This handles cases where on-hold tasks don't appear
	// in GanttBars (e.g., tasks with no milestone/package).
	projectInfo := &github.ProjectItemInfo{
		ProjectID: "proj-1",
		ItemID:    "item-1",
		FieldIDs: map[string]string{
			"Expected Start":      "field-1",
			"Expected Completion": "field-2",
			"98% Completion":      "field-3",
		},
	}

	// Issue is on-hold (via Scheduling Status) with dates set
	issues := map[string]issueWithProject{
		"github.com/owner/repo/issues/1": {
			owner:              "owner",
			repo:               "repo",
			issueNum:           1,
			title:              "On Hold Task With Dates",
			state:              "open",
			schedulingStatus:   "On Hold",
			project:            projectInfo,
			hasSchedulingDates: true,
		},
	}

	// Empty GanttData - simulates on-hold task not appearing in scheduler output
	// (e.g., because it has no milestone and no other tasks in its package)
	ganttData := planner.GanttData{
		Bars: []planner.GanttBar{},
	}

	updates := prepareUpdates(ganttData, issues)

	// Should have 1 update - on-hold task with dates should be cleared
	// even though it's not in the GanttBars
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}

	if updates[0].issueNum != 1 {
		t.Errorf("expected issue #1, got #%d", updates[0].issueNum)
	}
	if !updates[0].clearDates {
		t.Error("expected clearDates=true")
	}
	if updates[0].clearReason != "on hold" {
		t.Errorf("expected clearReason=%q, got %q", "on hold", updates[0].clearReason)
	}
}

func TestPrepareUpdates_ClosedDetectedFromGitHubNotGantt(t *testing.T) {
	// Closed status should be detected from GitHub's state field,
	// not from the GanttBar's Done field.
	projectInfo := &github.ProjectItemInfo{
		ProjectID: "proj-1",
		ItemID:    "item-1",
		FieldIDs: map[string]string{
			"Expected Start":      "field-1",
			"Expected Completion": "field-2",
			"98% Completion":      "field-3",
			"Low Estimate":        "field-4",
			"High Estimate":       "field-5",
		},
	}

	// Issue is closed with estimates set
	issues := map[string]issueWithProject{
		"github.com/owner/repo/issues/1": {
			owner:        "owner",
			repo:         "repo",
			issueNum:     1,
			title:        "Closed Task With Estimates",
			state:        "closed",
			project:      projectInfo,
			hasEstimates: true,
		},
	}

	// Empty GanttData - simulates closed task not appearing in scheduler output
	ganttData := planner.GanttData{
		Bars: []planner.GanttBar{},
	}

	updates := prepareUpdates(ganttData, issues)

	// Should have 1 update - closed task with estimates should be cleared
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}

	if !updates[0].clearDates {
		t.Error("expected clearDates=true")
	}
	if updates[0].clearReason != "closed" {
		t.Errorf("expected clearReason=%q, got %q", "closed", updates[0].clearReason)
	}
}

func TestPrepareUpdates_DifferentReposSameIssueNumber(t *testing.T) {
	// Bug fix test: issues from different repos with the same issue number
	// should be tracked separately. Previously, closing aer-dist#2 would
	// incorrectly mark a2b#2 as processed.
	projectInfo := &github.ProjectItemInfo{
		ProjectID: "proj-1",
		ItemID:    "item-1",
		FieldIDs: map[string]string{
			"Expected Start":      "field-1",
			"Expected Completion": "field-2",
			"98% Completion":      "field-3",
		},
	}

	// Two issues from different repos with same issue number
	issues := map[string]issueWithProject{
		"github.com/owner/repo-a/issues/2": {
			owner:              "owner",
			repo:               "repo-a",
			issueNum:           2,
			title:              "Closed Task In Repo A",
			state:              "closed",
			project:            projectInfo,
			hasSchedulingDates: true,
		},
		"github.com/owner/repo-b/issues/2": {
			owner:    "owner",
			repo:     "repo-b",
			issueNum: 2,
			title:    "Open Task In Repo B",
			state:    "open",
			project:  projectInfo,
		},
	}

	ganttData := planner.GanttData{
		Bars: []planner.GanttBar{
			{
				ID:   "owner/repo-a#2",
				Name: "Closed Task In Repo A",
				Done: true,
			},
			{
				ID:   "owner/repo-b#2",
				Name: "Open Task In Repo B",
			},
		},
	}

	updates := prepareUpdates(ganttData, issues)

	// Should have 2 updates: one to clear dates for closed task,
	// one to set dates for open task
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}

	// Find updates by repo
	var repoAUpdate, repoBUpdate *dateUpdate
	for i := range updates {
		if updates[i].repo == "repo-a" {
			repoAUpdate = &updates[i]
		} else if updates[i].repo == "repo-b" {
			repoBUpdate = &updates[i]
		}
	}

	if repoAUpdate == nil {
		t.Fatal("expected to find repo-a update")
	}
	if !repoAUpdate.clearDates {
		t.Error("repo-a task should have clearDates=true (it's closed)")
	}

	if repoBUpdate == nil {
		t.Fatal("expected to find repo-b update")
	}
	if repoBUpdate.clearDates {
		t.Error("repo-b task should have clearDates=false (it's open)")
	}
}

func TestPrepareUpdates_MultipleReposWithOverlappingIssueNumbers(t *testing.T) {
	// More comprehensive test: multiple repos with overlapping issue numbers
	// in various states
	projectInfo := &github.ProjectItemInfo{
		ProjectID: "proj-1",
		ItemID:    "item-1",
		FieldIDs: map[string]string{
			"Expected Start":      "field-1",
			"Expected Completion": "field-2",
			"98% Completion":      "field-3",
		},
	}

	issues := map[string]issueWithProject{
		// aer-dist repo: issues 1, 2, 4 all closed
		"github.com/owner/aer-dist/issues/1": {
			owner:              "owner",
			repo:               "aer-dist",
			issueNum:           1,
			title:              "aer-dist #1 (closed)",
			state:              "closed",
			project:            projectInfo,
			hasSchedulingDates: true,
		},
		"github.com/owner/aer-dist/issues/2": {
			owner:              "owner",
			repo:               "aer-dist",
			issueNum:           2,
			title:              "aer-dist #2 (closed)",
			state:              "closed",
			project:            projectInfo,
			hasSchedulingDates: true,
		},
		"github.com/owner/aer-dist/issues/4": {
			owner:              "owner",
			repo:               "aer-dist",
			issueNum:           4,
			title:              "aer-dist #4 (closed)",
			state:              "closed",
			project:            projectInfo,
			hasSchedulingDates: true,
		},
		// a2b repo: issues 2, 4 open
		"github.com/owner/a2b/issues/2": {
			owner:    "owner",
			repo:     "a2b",
			issueNum: 2,
			title:    "a2b #2 (open)",
			state:    "open",
			project:  projectInfo,
		},
		"github.com/owner/a2b/issues/4": {
			owner:    "owner",
			repo:     "a2b",
			issueNum: 4,
			title:    "a2b #4 (open)",
			state:    "open",
			project:  projectInfo,
		},
	}

	ganttData := planner.GanttData{
		Bars: []planner.GanttBar{
			{ID: "owner/aer-dist#1", Name: "aer-dist #1 (closed)", Done: true},
			{ID: "owner/aer-dist#2", Name: "aer-dist #2 (closed)", Done: true},
			{ID: "owner/aer-dist#4", Name: "aer-dist #4 (closed)", Done: true},
			{ID: "owner/a2b#2", Name: "a2b #2 (open)"},
			{ID: "owner/a2b#4", Name: "a2b #4 (open)"},
		},
	}

	updates := prepareUpdates(ganttData, issues)

	// Should have 5 updates total
	if len(updates) != 5 {
		t.Fatalf("expected 5 updates, got %d", len(updates))
	}

	// Count updates by type
	clearCount := 0
	setCount := 0
	for _, u := range updates {
		if u.clearDates {
			clearCount++
		} else {
			setCount++
		}
	}

	// 3 closed tasks should have clearDates=true
	if clearCount != 3 {
		t.Errorf("expected 3 updates with clearDates=true, got %d", clearCount)
	}

	// 2 open tasks should have clearDates=false
	if setCount != 2 {
		t.Errorf("expected 2 updates with clearDates=false, got %d", setCount)
	}

	// Verify a2b tasks specifically are not marked for clearing
	for _, u := range updates {
		if u.repo == "a2b" && u.clearDates {
			t.Errorf("a2b #%d should not have clearDates=true", u.issueNum)
		}
	}
}

func TestProjectInfoFromGetProjectFields(t *testing.T) {
	// Test that ProjectItemInfo can be constructed with item ID from GetProjectItems
	// and field IDs from GetProjectFields
	projectFields := &github.ProjectItemInfo{
		ProjectID: "proj-123",
		FieldIDs: map[string]string{
			"Expected Start":      "field-1",
			"Expected Completion": "field-2",
			"98% Completion":      "field-3",
			"Low Estimate":        "field-4",
			"High Estimate":       "field-5",
		},
		SingleSelectOptions: map[string]map[string]string{
			"Scheduling Status": {
				"On Hold": "opt-1",
				"Active":  "opt-2",
			},
		},
	}

	// Simulate what fetchProjectItems does: combine project fields with item ID
	itemID := "item-456"
	projectInfo := &github.ProjectItemInfo{
		ProjectID:           projectFields.ProjectID,
		ItemID:              itemID,
		FieldIDs:            projectFields.FieldIDs,
		SingleSelectOptions: projectFields.SingleSelectOptions,
	}

	// Verify all fields are accessible
	if projectInfo.ProjectID != "proj-123" {
		t.Errorf("ProjectID = %q, want %q", projectInfo.ProjectID, "proj-123")
	}
	if projectInfo.ItemID != "item-456" {
		t.Errorf("ItemID = %q, want %q", projectInfo.ItemID, "item-456")
	}
	if len(projectInfo.FieldIDs) != 5 {
		t.Errorf("FieldIDs has %d entries, want 5", len(projectInfo.FieldIDs))
	}
	if projectInfo.FieldIDs["Low Estimate"] != "field-4" {
		t.Errorf("Low Estimate field ID = %q, want %q", projectInfo.FieldIDs["Low Estimate"], "field-4")
	}
}
