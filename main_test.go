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

	if activeUpdate == nil {
		t.Fatal("expected to find active task update")
	}
	if activeUpdate.clearDates {
		t.Error("expected active task to have clearDates=false")
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
			owner:              "owner",
			repo:               "repo",
			issueNum:           1,
			title:              "Closed Task With Estimates",
			state:              "closed",
			project:            projectInfo,
			lowEstimate:        2,
			highEstimate:       8,
			hasSchedulingDates: true, // has estimates to clear
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
