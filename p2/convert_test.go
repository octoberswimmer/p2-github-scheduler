package p2

import (
	"strings"
	"testing"

	"github.com/octoberswimmer/p2/github"
	"github.com/octoberswimmer/p2/planner"
)

// ptr returns a pointer to the given float64 value
func ptr(f float64) *float64 {
	return &f
}

func TestIssuesToTasks_OnHoldViaSchedulingStatus(t *testing.T) {
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:            "owner",
			Repo:             "repo",
			IssueNum:         1,
			Title:            "On Hold Task",
			State:            "open",
			SchedulingStatus: "On Hold",
		},
		"github.com/owner/repo/issues/2": {
			Owner:            "owner",
			Repo:             "repo",
			IssueNum:         2,
			Title:            "Active Task",
			State:            "open",
			SchedulingStatus: "Active",
		},
		"github.com/owner/repo/issues/3": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 3,
			Title:    "No Status Task",
			State:    "open",
		},
	}

	tasks, _, _ := IssuesToTasks(issues, nil)

	// Find tasks by title
	taskMap := make(map[string]planner.Task)
	for _, task := range tasks {
		taskMap[task.Name] = task
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
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 1,
			Title:    "Regular Issue",
			State:    "open",
		},
		"draft:PVTI_abc123": {
			Title:         "Draft Issue",
			IsDraft:       true,
			ProjectItemID: "PVTI_abc123",
		},
	}

	tasks, _, _ := IssuesToTasks(issues, nil)

	// Find tasks by title
	taskMap := make(map[string]planner.Task)
	for _, task := range tasks {
		taskMap[task.Name] = task
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

func TestIssuesToTasks_PreservesOrder(t *testing.T) {
	// Create issues with specific order values (simulating GitHub Project order)
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/5": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 5,
			Title:    "Third Task",
			State:    "open",
			Order:    2,
		},
		"github.com/owner/repo/issues/1": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 1,
			Title:    "First Task",
			State:    "open",
			Order:    0,
		},
		"github.com/owner/repo/issues/10": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 10,
			Title:    "Second Task",
			State:    "open",
			Order:    1,
		},
	}

	tasks, _, _ := IssuesToTasks(issues, nil)

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
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 1,
			Title:    "Blocker Task",
			State:    "open",
			Order:    0,
		},
		"github.com/owner/repo/issues/2": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 2,
			Title:    "Blocked Task",
			State:    "open",
			Order:    1,
			BlockedBy: []github.IssueRef{
				{Owner: "owner", Repo: "repo", Number: 1},     // accessible
				{Owner: "other", Repo: "private", Number: 99}, // inaccessible
			},
		},
	}

	tasks, _, _ := IssuesToTasks(issues, nil)

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
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 1,
			Title:    "Assigned Task",
			State:    "open",
			Assignee: "cwarden",
		},
		"github.com/owner/repo/issues/2": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 2,
			Title:    "Unassigned Task",
			State:    "open",
			// no assignee
		},
	}

	tasks, users, _ := IssuesToTasks(issues, nil)

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

func TestIssuesToTasks_DetectsMissingDependencies(t *testing.T) {
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:        "owner",
			Repo:         "repo",
			IssueNum:     1,
			Title:        "Task with missing dep",
			State:        "open",
			LowEstimate:  ptr(2),
			HighEstimate: ptr(4),
			BlockedBy: []github.IssueRef{
				{Owner: "other", Repo: "missing", Number: 99},
			},
		},
		"github.com/owner/repo/issues/2": {
			Owner:        "owner",
			Repo:         "repo",
			IssueNum:     2,
			Title:        "Task with valid dep",
			State:        "open",
			LowEstimate:  ptr(2),
			HighEstimate: ptr(4),
			BlockedBy: []github.IssueRef{
				{Owner: "owner", Repo: "repo", Number: 1},
			},
		},
	}

	_, _, schedIssues := IssuesToTasks(issues, nil)

	// Should have 1 scheduling issue for missing dependency
	if len(schedIssues) != 1 {
		t.Fatalf("expected 1 scheduling issue, got %d", len(schedIssues))
	}

	si := schedIssues[0]
	if si.Reason != "missing_dependency" {
		t.Errorf("expected reason 'missing_dependency', got %q", si.Reason)
	}
	if si.IssueNum != 1 {
		t.Errorf("expected issue #1, got #%d", si.IssueNum)
	}
	if len(si.Details) != 1 || si.Details[0] != "other/missing#99" {
		t.Errorf("expected details ['other/missing#99'], got %v", si.Details)
	}
}

func TestIssuesToTasks_DetectsOnHoldDependencies(t *testing.T) {
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:            "owner",
			Repo:             "repo",
			IssueNum:         1,
			Title:            "On-hold task",
			State:            "open",
			SchedulingStatus: "On Hold",
			LowEstimate:      ptr(2),
			HighEstimate:     ptr(4),
		},
		"github.com/owner/repo/issues/2": {
			Owner:        "owner",
			Repo:         "repo",
			IssueNum:     2,
			Title:        "Task depending on on-hold",
			State:        "open",
			LowEstimate:  ptr(2),
			HighEstimate: ptr(4),
			BlockedBy: []github.IssueRef{
				{Owner: "owner", Repo: "repo", Number: 1},
			},
		},
	}

	tasks, _, schedIssues := IssuesToTasks(issues, nil)

	// Should have 1 scheduling issue for on-hold dependency
	if len(schedIssues) != 1 {
		t.Fatalf("expected 1 scheduling issue, got %d", len(schedIssues))
	}

	si := schedIssues[0]
	if si.Reason != "onhold_dependency" {
		t.Errorf("expected reason 'onhold_dependency', got %q", si.Reason)
	}
	if si.IssueNum != 2 {
		t.Errorf("expected issue #2, got #%d", si.IssueNum)
	}

	// The on-hold dependency should NOT be in DependsOn
	var task2 planner.Task
	for _, task := range tasks {
		if task.ID == "owner/repo#2" {
			task2 = task
			break
		}
	}
	if len(task2.DependsOn) != 0 {
		t.Errorf("expected no DependsOn for task with on-hold dep, got %v", task2.DependsOn)
	}
}

func TestIssuesToTasks_ClosedDependencySkipped(t *testing.T) {
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:            "owner",
			Repo:             "repo",
			IssueNum:         1,
			Title:            "Closed on-hold task",
			State:            "closed",
			SchedulingStatus: "On Hold",
			LowEstimate:      ptr(2),
			HighEstimate:     ptr(4),
		},
		"github.com/owner/repo/issues/2": {
			Owner:        "owner",
			Repo:         "repo",
			IssueNum:     2,
			Title:        "Task depending on closed",
			State:        "open",
			LowEstimate:  ptr(2),
			HighEstimate: ptr(4),
			BlockedBy: []github.IssueRef{
				{Owner: "owner", Repo: "repo", Number: 1},
			},
		},
	}

	tasks, _, schedIssues := IssuesToTasks(issues, nil)

	// Should have NO scheduling issues - closed deps are satisfied
	if len(schedIssues) != 0 {
		t.Errorf("expected 0 scheduling issues, got %d: %+v", len(schedIssues), schedIssues)
	}

	// Closed dependencies should NOT be in DependsOn (they're already satisfied)
	var task2 planner.Task
	for _, task := range tasks {
		if task.ID == "owner/repo#2" {
			task2 = task
			break
		}
	}
	if len(task2.DependsOn) != 0 {
		t.Errorf("expected 0 DependsOn for task with closed dep (satisfied), got %v", task2.DependsOn)
	}
}

func TestIssuesToTasks_OnHoldTaskNoSchedulingIssue(t *testing.T) {
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:        "owner",
			Repo:         "repo",
			IssueNum:     1,
			Title:        "Missing dep task",
			State:        "open",
			LowEstimate:  ptr(2),
			HighEstimate: ptr(4),
			BlockedBy: []github.IssueRef{
				{Owner: "other", Repo: "missing", Number: 99},
			},
		},
		"github.com/owner/repo/issues/2": {
			Owner:            "owner",
			Repo:             "repo",
			IssueNum:         2,
			Title:            "On-hold with missing dep",
			State:            "open",
			SchedulingStatus: "On Hold",
			LowEstimate:      ptr(2),
			HighEstimate:     ptr(4),
			BlockedBy: []github.IssueRef{
				{Owner: "other", Repo: "missing", Number: 99},
			},
		},
	}

	_, _, schedIssues := IssuesToTasks(issues, nil)

	// Should have only 1 scheduling issue - the on-hold task doesn't get one
	if len(schedIssues) != 1 {
		t.Fatalf("expected 1 scheduling issue, got %d", len(schedIssues))
	}
	if schedIssues[0].IssueNum != 1 {
		t.Errorf("expected scheduling issue for #1, got #%d", schedIssues[0].IssueNum)
	}
}

func TestIssuesToTasks_DetectsInaccessibleBlockers(t *testing.T) {
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:                "owner",
			Repo:                 "repo",
			IssueNum:             1,
			Title:                "Task with inaccessible blocker",
			State:                "open",
			LowEstimate:          ptr(2),
			HighEstimate:         ptr(4),
			InaccessibleBlockers: 2,
		},
		"github.com/owner/repo/issues/2": {
			Owner:        "owner",
			Repo:         "repo",
			IssueNum:     2,
			Title:        "Normal task",
			State:        "open",
			LowEstimate:  ptr(2),
			HighEstimate: ptr(4),
		},
	}

	_, _, schedIssues := IssuesToTasks(issues, nil)

	// Should have 1 scheduling issue for inaccessible blockers
	if len(schedIssues) != 1 {
		t.Fatalf("expected 1 scheduling issue, got %d", len(schedIssues))
	}

	si := schedIssues[0]
	if si.Reason != "inaccessible_dependency" {
		t.Errorf("expected reason 'inaccessible_dependency', got %q", si.Reason)
	}
	if si.IssueNum != 1 {
		t.Errorf("expected issue #1, got #%d", si.IssueNum)
	}
	if !strings.Contains(si.Details[0], "2 blocker(s)") {
		t.Errorf("expected details to mention '2 blocker(s)', got %v", si.Details)
	}
}

func TestIssuesToTasks_DetectsMissingLowEstimate(t *testing.T) {
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:        "owner",
			Repo:         "repo",
			IssueNum:     1,
			Title:        "Task with missing low estimate",
			State:        "open",
			LowEstimate:  nil,
			HighEstimate: ptr(8),
		},
	}

	_, _, schedIssues := IssuesToTasks(issues, nil)

	if len(schedIssues) != 1 {
		t.Fatalf("expected 1 scheduling issue, got %d", len(schedIssues))
	}

	si := schedIssues[0]
	if si.Reason != "missing_estimate" {
		t.Errorf("expected reason 'missing_estimate', got %q", si.Reason)
	}
	if len(si.Details) != 1 || si.Details[0] != "Low Estimate" {
		t.Errorf("expected details ['Low Estimate'], got %v", si.Details)
	}
}

func TestIssuesToTasks_DetectsMissingHighEstimate(t *testing.T) {
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:        "owner",
			Repo:         "repo",
			IssueNum:     1,
			Title:        "Task with missing high estimate",
			State:        "open",
			LowEstimate:  ptr(4),
			HighEstimate: nil,
		},
	}

	_, _, schedIssues := IssuesToTasks(issues, nil)

	if len(schedIssues) != 1 {
		t.Fatalf("expected 1 scheduling issue, got %d", len(schedIssues))
	}

	si := schedIssues[0]
	if si.Reason != "missing_estimate" {
		t.Errorf("expected reason 'missing_estimate', got %q", si.Reason)
	}
	if len(si.Details) != 1 || si.Details[0] != "High Estimate" {
		t.Errorf("expected details ['High Estimate'], got %v", si.Details)
	}
}

func TestIssuesToTasks_DetectsInvalidEstimate(t *testing.T) {
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:        "owner",
			Repo:         "repo",
			IssueNum:     1,
			Title:        "Task with invalid estimates",
			State:        "open",
			LowEstimate:  ptr(8),
			HighEstimate: ptr(4),
		},
	}

	_, _, schedIssues := IssuesToTasks(issues, nil)

	if len(schedIssues) != 1 {
		t.Fatalf("expected 1 scheduling issue, got %d", len(schedIssues))
	}

	si := schedIssues[0]
	if si.Reason != "invalid_estimate" {
		t.Errorf("expected reason 'invalid_estimate', got %q", si.Reason)
	}
	if len(si.Details) != 1 {
		t.Errorf("expected 1 detail, got %d", len(si.Details))
	}
	if !strings.Contains(si.Details[0], "must be greater than or equal to") {
		t.Errorf("expected detail to mention 'must be greater than or equal to', got %q", si.Details[0])
	}
}

func TestIssuesToTasks_ValidEstimatesNoIssue(t *testing.T) {
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:        "owner",
			Repo:         "repo",
			IssueNum:     1,
			Title:        "Task with valid estimates",
			State:        "open",
			LowEstimate:  ptr(4),
			HighEstimate: ptr(8),
		},
	}

	_, _, schedIssues := IssuesToTasks(issues, nil)

	if len(schedIssues) != 0 {
		t.Errorf("expected 0 scheduling issues, got %d: %+v", len(schedIssues), schedIssues)
	}
}

func TestIssuesToTasks_EqualEstimatesValid(t *testing.T) {
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:        "owner",
			Repo:         "repo",
			IssueNum:     1,
			Title:        "Task with equal estimates",
			State:        "open",
			LowEstimate:  ptr(4),
			HighEstimate: ptr(4),
		},
	}

	_, _, schedIssues := IssuesToTasks(issues, nil)

	if len(schedIssues) != 0 {
		t.Errorf("expected 0 scheduling issues (equal estimates are valid), got %d: %+v", len(schedIssues), schedIssues)
	}
}

func TestIssuesToTasks_ZeroEstimatesValid(t *testing.T) {
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:        "owner",
			Repo:         "repo",
			IssueNum:     1,
			Title:        "Task with zero estimates",
			State:        "open",
			LowEstimate:  ptr(0),
			HighEstimate: ptr(0),
		},
	}

	_, _, schedIssues := IssuesToTasks(issues, nil)

	if len(schedIssues) != 0 {
		t.Errorf("expected 0 scheduling issues (zero is a valid estimate), got %d: %+v", len(schedIssues), schedIssues)
	}
}

func TestIssuesToTasks_OnHoldTaskNoEstimateIssue(t *testing.T) {
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:            "owner",
			Repo:             "repo",
			IssueNum:         1,
			Title:            "On-hold task with missing estimate",
			State:            "open",
			SchedulingStatus: "On Hold",
			LowEstimate:      nil,
			HighEstimate:     ptr(8),
		},
	}

	_, _, schedIssues := IssuesToTasks(issues, nil)

	if len(schedIssues) != 0 {
		t.Errorf("expected 0 scheduling issues for on-hold task, got %d: %+v", len(schedIssues), schedIssues)
	}
}

func TestIssuesToTasks_ClosedTaskNoEstimateIssue(t *testing.T) {
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:        "owner",
			Repo:         "repo",
			IssueNum:     1,
			Title:        "Closed task with invalid estimates",
			State:        "closed",
			LowEstimate:  ptr(8),
			HighEstimate: ptr(4),
		},
	}

	_, _, schedIssues := IssuesToTasks(issues, nil)

	if len(schedIssues) != 0 {
		t.Errorf("expected 0 scheduling issues for closed task, got %d: %+v", len(schedIssues), schedIssues)
	}
}

func TestIssuesToTasks_DraftTaskNoEstimateIssue(t *testing.T) {
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/draft/item-123": {
			Owner:         "owner",
			Repo:          "repo",
			Title:         "Draft task with invalid estimates",
			State:         "open",
			IsDraft:       true,
			ProjectItemID: "item-123",
			LowEstimate:   ptr(8),
			HighEstimate:  ptr(4),
		},
	}

	_, _, schedIssues := IssuesToTasks(issues, nil)

	if len(schedIssues) != 0 {
		t.Errorf("expected 0 scheduling issues for draft task, got %d: %+v", len(schedIssues), schedIssues)
	}
}

func TestIssuesToTasks_BothEstimatesNilReportsMissingAndGetsDefaults(t *testing.T) {
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:        "owner",
			Repo:         "repo",
			IssueNum:     1,
			Title:        "Task with no estimates",
			State:        "open",
			LowEstimate:  nil,
			HighEstimate: nil,
		},
	}

	tasks, _, schedIssues := IssuesToTasks(issues, nil)

	// Should report both missing estimates
	if len(schedIssues) != 1 {
		t.Fatalf("expected 1 scheduling issue when both estimates are nil, got %d: %+v", len(schedIssues), schedIssues)
	}
	if schedIssues[0].Reason != "missing_estimate" {
		t.Errorf("expected reason 'missing_estimate', got %s", schedIssues[0].Reason)
	}
	if len(schedIssues[0].Details) != 2 {
		t.Errorf("expected 2 missing fields, got %d: %v", len(schedIssues[0].Details), schedIssues[0].Details)
	}

	// Verify defaults were still applied for scheduling
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].EstimateLow != 1 {
		t.Errorf("expected default low estimate of 1, got %.1f", tasks[0].EstimateLow)
	}
	if tasks[0].EstimateHigh != 4 {
		t.Errorf("expected default high estimate of 4, got %.1f", tasks[0].EstimateHigh)
	}
}
