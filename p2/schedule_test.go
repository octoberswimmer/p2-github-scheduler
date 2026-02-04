package p2

import (
	"testing"
	"time"

	"github.com/octoberswimmer/p2/github"
	"github.com/octoberswimmer/p2/planner"
)

var (
	testStart  = time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)
	testMean   = time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
	testEnd98  = time.Date(2026, 2, 15, 0, 0, 0, 0, time.UTC)
	otherStart = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	otherMean  = time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC)
	otherEnd98 = time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
)

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
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:              "owner",
			Repo:               "repo",
			IssueNum:           1,
			Title:              "On Hold Task",
			State:              "open",
			SchedulingStatus:   "On Hold",
			Project:            projectInfo,
			HasSchedulingDates: true,
		},
		"github.com/owner/repo/issues/2": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 2,
			Title:    "Active Task",
			State:    "open",
			Project:  projectInfo,
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
				ID:           "owner/repo#2",
				Name:         "Active Task",
				ExpStartDate: testStart,
				MeanDate:     testMean,
				End98Date:    testEnd98,
			},
		},
	}

	updates := PrepareUpdates(ganttData, issues, nil)

	// Should have 2 updates
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}

	// Find the on-hold task update
	var onHoldUpdate, activeUpdate *DateUpdate
	for i := range updates {
		if updates[i].IssueNum == 1 {
			onHoldUpdate = &updates[i]
		} else if updates[i].IssueNum == 2 {
			activeUpdate = &updates[i]
		}
	}

	if onHoldUpdate == nil {
		t.Fatal("expected to find on-hold task update")
	}
	if !onHoldUpdate.ClearDates {
		t.Error("expected on-hold task to have ClearDates=true")
	}
	if onHoldUpdate.ClearReason != "on hold" {
		t.Errorf("expected on-hold task ClearReason=%q, got %q", "on hold", onHoldUpdate.ClearReason)
	}

	if activeUpdate == nil {
		t.Fatal("expected to find active task update")
	}
	if activeUpdate.ClearDates {
		t.Error("expected active task to have ClearDates=false")
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

	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:            "owner",
			Repo:             "repo",
			IssueNum:         1,
			Title:            "On Hold With Only Estimates",
			State:            "open",
			SchedulingStatus: "On Hold",
			Project:          projectInfo,
			LowEstimate:      ptr(2),
			HighEstimate:     ptr(8),
			// HasSchedulingDates is false - no dates set
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

	updates := PrepareUpdates(ganttData, issues, nil)

	// Should have 0 updates - on-hold with only estimates should be skipped
	if len(updates) != 0 {
		t.Fatalf("expected 0 updates for on-hold task with only estimates, got %d", len(updates))
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
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:              "owner",
			Repo:               "repo",
			IssueNum:           1,
			Title:              "Closed Task",
			State:              "closed",
			Project:            projectInfo,
			HasSchedulingDates: true,
		},
		"github.com/owner/repo/issues/2": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 2,
			Title:    "Open Task",
			State:    "open",
			Project:  projectInfo,
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
				ID:           "owner/repo#2",
				Name:         "Open Task",
				ExpStartDate: testStart,
				MeanDate:     testMean,
				End98Date:    testEnd98,
			},
		},
	}

	updates := PrepareUpdates(ganttData, issues, nil)

	// Should have 2 updates
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}

	// Find the closed task update
	var closedUpdate, openUpdate *DateUpdate
	for i := range updates {
		if updates[i].IssueNum == 1 {
			closedUpdate = &updates[i]
		} else if updates[i].IssueNum == 2 {
			openUpdate = &updates[i]
		}
	}

	if closedUpdate == nil {
		t.Fatal("expected to find closed task update")
	}
	if !closedUpdate.ClearDates {
		t.Error("expected closed task to have ClearDates=true")
	}
	if closedUpdate.ClearReason != "closed" {
		t.Errorf("expected closed task ClearReason=%q, got %q", "closed", closedUpdate.ClearReason)
	}

	if openUpdate == nil {
		t.Fatal("expected to find open task update")
	}
	if openUpdate.ClearDates {
		t.Error("expected open task to have ClearDates=false")
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
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:              "owner",
			Repo:               "repo",
			IssueNum:           1,
			Title:              "Closed Task Without Dates",
			State:              "closed",
			Project:            projectInfo,
			HasSchedulingDates: false, // no dates to clear
		},
		"github.com/owner/repo/issues/2": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 2,
			Title:    "Open Task",
			State:    "open",
			Project:  projectInfo,
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
				ID:           "owner/repo#2",
				Name:         "Open Task",
				ExpStartDate: testStart,
				MeanDate:     testMean,
				End98Date:    testEnd98,
			},
		},
	}

	updates := PrepareUpdates(ganttData, issues, nil)

	// Should have only 1 update (the open task), closed task without dates is skipped
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}

	if updates[0].IssueNum != 2 {
		t.Errorf("expected open task update, got issue #%d", updates[0].IssueNum)
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
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:        "owner",
			Repo:         "repo",
			IssueNum:     1,
			Title:        "Closed Task With Estimates",
			State:        "closed",
			Project:      projectInfo,
			LowEstimate:  ptr(2),
			HighEstimate: ptr(8), // has estimates to clear (no dates)
		},
		"github.com/owner/repo/issues/2": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 2,
			Title:    "Open Task",
			State:    "open",
			Project:  projectInfo,
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
				ID:           "owner/repo#2",
				Name:         "Open Task",
				ExpStartDate: testStart,
				MeanDate:     testMean,
				End98Date:    testEnd98,
			},
		},
	}

	updates := PrepareUpdates(ganttData, issues, nil)

	// Should have 2 updates - closed task with estimates should be included
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}

	// Find the closed task update
	var closedUpdate *DateUpdate
	for i := range updates {
		if updates[i].IssueNum == 1 {
			closedUpdate = &updates[i]
			break
		}
	}

	if closedUpdate == nil {
		t.Fatal("expected to find closed task update")
	}
	if !closedUpdate.ClearDates {
		t.Error("expected closed task with estimates to have ClearDates=true")
	}
	if closedUpdate.ClearReason != "closed" {
		t.Errorf("expected closed task ClearReason=%q, got %q", "closed", closedUpdate.ClearReason)
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
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:              "owner",
			Repo:               "repo",
			IssueNum:           1,
			Title:              "On Hold Task With Dates",
			State:              "open",
			SchedulingStatus:   "On Hold",
			Project:            projectInfo,
			HasSchedulingDates: true,
		},
	}

	// Empty GanttData - simulates on-hold task not appearing in scheduler output
	// (e.g., because it has no milestone and no other tasks in its package)
	ganttData := planner.GanttData{
		Bars: []planner.GanttBar{},
	}

	updates := PrepareUpdates(ganttData, issues, nil)

	// Should have 1 update - on-hold task with dates should be cleared
	// even though it's not in the GanttBars
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}

	if updates[0].IssueNum != 1 {
		t.Errorf("expected issue #1, got #%d", updates[0].IssueNum)
	}
	if !updates[0].ClearDates {
		t.Error("expected ClearDates=true")
	}
	if updates[0].ClearReason != "on hold" {
		t.Errorf("expected ClearReason=%q, got %q", "on hold", updates[0].ClearReason)
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
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:        "owner",
			Repo:         "repo",
			IssueNum:     1,
			Title:        "Closed Task With Estimates",
			State:        "closed",
			Project:      projectInfo,
			LowEstimate:  ptr(2),
			HighEstimate: ptr(8),
		},
	}

	// Empty GanttData - simulates closed task not appearing in scheduler output
	ganttData := planner.GanttData{
		Bars: []planner.GanttBar{},
	}

	updates := PrepareUpdates(ganttData, issues, nil)

	// Should have 1 update - closed task with estimates should be cleared
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}

	if !updates[0].ClearDates {
		t.Error("expected ClearDates=true")
	}
	if updates[0].ClearReason != "closed" {
		t.Errorf("expected ClearReason=%q, got %q", "closed", updates[0].ClearReason)
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
	issues := map[string]IssueWithProject{
		"github.com/owner/repo-a/issues/2": {
			Owner:              "owner",
			Repo:               "repo-a",
			IssueNum:           2,
			Title:              "Closed Task In Repo A",
			State:              "closed",
			Project:            projectInfo,
			HasSchedulingDates: true,
		},
		"github.com/owner/repo-b/issues/2": {
			Owner:    "owner",
			Repo:     "repo-b",
			IssueNum: 2,
			Title:    "Open Task In Repo B",
			State:    "open",
			Project:  projectInfo,
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
				ID:           "owner/repo-b#2",
				Name:         "Open Task In Repo B",
				ExpStartDate: testStart,
				MeanDate:     testMean,
				End98Date:    testEnd98,
			},
		},
	}

	updates := PrepareUpdates(ganttData, issues, nil)

	// Should have 2 updates: one to clear dates for closed task,
	// one to set dates for open task
	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}

	// Find updates by repo
	var repoAUpdate, repoBUpdate *DateUpdate
	for i := range updates {
		if updates[i].Repo == "repo-a" {
			repoAUpdate = &updates[i]
		} else if updates[i].Repo == "repo-b" {
			repoBUpdate = &updates[i]
		}
	}

	if repoAUpdate == nil {
		t.Fatal("expected to find repo-a update")
	}
	if !repoAUpdate.ClearDates {
		t.Error("repo-a task should have ClearDates=true (it's closed)")
	}

	if repoBUpdate == nil {
		t.Fatal("expected to find repo-b update")
	}
	if repoBUpdate.ClearDates {
		t.Error("repo-b task should have ClearDates=false (it's open)")
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

	issues := map[string]IssueWithProject{
		// aer-dist repo: issues 1, 2, 4 all closed
		"github.com/owner/aer-dist/issues/1": {
			Owner:              "owner",
			Repo:               "aer-dist",
			IssueNum:           1,
			Title:              "aer-dist #1 (closed)",
			State:              "closed",
			Project:            projectInfo,
			HasSchedulingDates: true,
		},
		"github.com/owner/aer-dist/issues/2": {
			Owner:              "owner",
			Repo:               "aer-dist",
			IssueNum:           2,
			Title:              "aer-dist #2 (closed)",
			State:              "closed",
			Project:            projectInfo,
			HasSchedulingDates: true,
		},
		"github.com/owner/aer-dist/issues/4": {
			Owner:              "owner",
			Repo:               "aer-dist",
			IssueNum:           4,
			Title:              "aer-dist #4 (closed)",
			State:              "closed",
			Project:            projectInfo,
			HasSchedulingDates: true,
		},
		// a2b repo: issues 2, 4 open
		"github.com/owner/a2b/issues/2": {
			Owner:    "owner",
			Repo:     "a2b",
			IssueNum: 2,
			Title:    "a2b #2 (open)",
			State:    "open",
			Project:  projectInfo,
		},
		"github.com/owner/a2b/issues/4": {
			Owner:    "owner",
			Repo:     "a2b",
			IssueNum: 4,
			Title:    "a2b #4 (open)",
			State:    "open",
			Project:  projectInfo,
		},
	}

	ganttData := planner.GanttData{
		Bars: []planner.GanttBar{
			{ID: "owner/aer-dist#1", Name: "aer-dist #1 (closed)", Done: true},
			{ID: "owner/aer-dist#2", Name: "aer-dist #2 (closed)", Done: true},
			{ID: "owner/aer-dist#4", Name: "aer-dist #4 (closed)", Done: true},
			{ID: "owner/a2b#2", Name: "a2b #2 (open)", ExpStartDate: testStart, MeanDate: testMean, End98Date: testEnd98},
			{ID: "owner/a2b#4", Name: "a2b #4 (open)", ExpStartDate: otherStart, MeanDate: otherMean, End98Date: otherEnd98},
		},
	}

	updates := PrepareUpdates(ganttData, issues, nil)

	// Should have 5 updates total
	if len(updates) != 5 {
		t.Fatalf("expected 5 updates, got %d", len(updates))
	}

	// Count updates by type
	clearCount := 0
	setCount := 0
	for _, u := range updates {
		if u.ClearDates {
			clearCount++
		} else {
			setCount++
		}
	}

	// 3 closed tasks should have ClearDates=true
	if clearCount != 3 {
		t.Errorf("expected 3 updates with ClearDates=true, got %d", clearCount)
	}

	// 2 open tasks should have ClearDates=false
	if setCount != 2 {
		t.Errorf("expected 2 updates with ClearDates=false, got %d", setCount)
	}

	// Verify a2b tasks specifically are not marked for clearing
	for _, u := range updates {
		if u.Repo == "a2b" && u.ClearDates {
			t.Errorf("a2b #%d should not have ClearDates=true", u.IssueNum)
		}
	}
}

func TestExtractCycleIssues(t *testing.T) {
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 1,
			Title:    "Task in cycle",
		},
		"github.com/owner/repo/issues/2": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 2,
			Title:    "Another task in cycle",
		},
	}

	entries := planner.ScheduledEntries{
		Entries: []planner.ScheduledEntry{
			{ID: "owner/repo#1", Name: "Task in cycle", Cycle: []string{"owner/repo#1", "owner/repo#2", "owner/repo#1"}},
			{ID: "owner/repo#2", Name: "Another task in cycle", Cycle: []string{"owner/repo#2", "owner/repo#1", "owner/repo#2"}},
		},
	}

	schedIssues := ExtractCycleIssues(entries, issues, nil)

	if len(schedIssues) != 2 {
		t.Fatalf("expected 2 scheduling issues, got %d", len(schedIssues))
	}

	for _, si := range schedIssues {
		if si.Reason != "cycle" {
			t.Errorf("expected reason 'cycle', got %q", si.Reason)
		}
		if len(si.Details) == 0 {
			t.Errorf("expected cycle path in details")
		}
	}
}

func TestExtractCycleIssues_NoDuplicates(t *testing.T) {
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 1,
			Title:    "Task with missing dep and cycle",
		},
	}

	// Pre-existing scheduling issue for missing dep
	existing := []SchedulingIssue{
		{
			IssueRef: "github.com/owner/repo/issues/1",
			IssueNum: 1,
			Reason:   "missing_dependency",
		},
	}

	entries := planner.ScheduledEntries{
		Entries: []planner.ScheduledEntry{
			{ID: "owner/repo#1", Name: "Task", Cycle: []string{"owner/repo#1", "owner/repo#2", "owner/repo#1"}},
		},
	}

	schedIssues := ExtractCycleIssues(entries, issues, existing)

	// Should NOT add another issue for the same task
	if len(schedIssues) != 1 {
		t.Fatalf("expected 1 scheduling issue (no duplicate), got %d", len(schedIssues))
	}
}

func TestPrepareUpdates_UnschedulableIssuesClearDates(t *testing.T) {
	projectInfo := &github.ProjectItemInfo{
		ProjectID: "proj-1",
		ItemID:    "item-1",
		FieldIDs: map[string]string{
			"Expected Start":      "field-1",
			"Expected Completion": "field-2",
			"98% Completion":      "field-3",
		},
	}

	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:              "owner",
			Repo:               "repo",
			IssueNum:           1,
			Title:              "Unschedulable task",
			State:              "open",
			Project:            projectInfo,
			HasSchedulingDates: true,
		},
	}

	ganttData := planner.GanttData{}

	unschedulable := map[string]bool{
		"github.com/owner/repo/issues/1": true,
	}

	updates := PrepareUpdates(ganttData, issues, unschedulable)

	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}

	if !updates[0].ClearDates {
		t.Error("expected ClearDates=true for unschedulable issue")
	}
	if updates[0].ClearReason != "unschedulable" {
		t.Errorf("expected ClearReason='unschedulable', got %q", updates[0].ClearReason)
	}
}

func TestPrepareUpdates_UnschedulableNoDatesNoClear(t *testing.T) {
	projectInfo := &github.ProjectItemInfo{
		ProjectID: "proj-1",
		ItemID:    "item-1",
		FieldIDs: map[string]string{
			"Expected Start":      "field-1",
			"Expected Completion": "field-2",
			"98% Completion":      "field-3",
		},
	}

	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:              "owner",
			Repo:               "repo",
			IssueNum:           1,
			Title:              "Unschedulable task without dates",
			State:              "open",
			Project:            projectInfo,
			HasSchedulingDates: false, // No dates to clear
		},
	}

	ganttData := planner.GanttData{}

	unschedulable := map[string]bool{
		"github.com/owner/repo/issues/1": true,
	}

	updates := PrepareUpdates(ganttData, issues, unschedulable)

	// Should have no updates - no dates to clear
	if len(updates) != 0 {
		t.Fatalf("expected 0 updates (no dates to clear), got %d", len(updates))
	}
}

func TestPrepareUpdates_UnchangedDatesSkipped(t *testing.T) {
	projectInfo := &github.ProjectItemInfo{
		ProjectID: "proj-1",
		ItemID:    "item-1",
		FieldIDs: map[string]string{
			"Expected Start":      "field-1",
			"Expected Completion": "field-2",
			"98% Completion":      "field-3",
		},
	}

	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:              "owner",
			Repo:               "repo",
			IssueNum:           1,
			Title:              "Task with unchanged dates",
			State:              "open",
			Project:            projectInfo,
			HasSchedulingDates: true,
			ExpectedStart:      &testStart,
			ExpectedCompletion: &testMean,
			Completion98:       &testEnd98,
		},
		"github.com/owner/repo/issues/2": {
			Owner:              "owner",
			Repo:               "repo",
			IssueNum:           2,
			Title:              "Task with changed dates",
			State:              "open",
			Project:            projectInfo,
			HasSchedulingDates: true,
			ExpectedStart:      &testStart,
			ExpectedCompletion: &testMean,
			Completion98:       &testEnd98,
		},
	}

	ganttData := planner.GanttData{
		Bars: []planner.GanttBar{
			{
				ID:           "owner/repo#1",
				Name:         "Task with unchanged dates",
				ExpStartDate: testStart,
				MeanDate:     testMean,
				End98Date:    testEnd98,
			},
			{
				ID:           "owner/repo#2",
				Name:         "Task with changed dates",
				ExpStartDate: otherStart,
				MeanDate:     otherMean,
				End98Date:    otherEnd98,
			},
		},
	}

	updates := PrepareUpdates(ganttData, issues, nil)

	// Should have only 1 update - the task with changed dates
	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}

	if updates[0].IssueNum != 2 {
		t.Errorf("expected update for issue #2, got #%d", updates[0].IssueNum)
	}
}

func TestPrepareUpdates_NewDatesNotSkipped(t *testing.T) {
	projectInfo := &github.ProjectItemInfo{
		ProjectID: "proj-1",
		ItemID:    "item-1",
		FieldIDs: map[string]string{
			"Expected Start":      "field-1",
			"Expected Completion": "field-2",
			"98% Completion":      "field-3",
		},
	}

	// Issue has no existing dates
	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 1,
			Title:    "Task without dates",
			State:    "open",
			Project:  projectInfo,
		},
	}

	ganttData := planner.GanttData{
		Bars: []planner.GanttBar{
			{
				ID:           "owner/repo#1",
				Name:         "Task without dates",
				ExpStartDate: testStart,
				MeanDate:     testMean,
				End98Date:    testEnd98,
			},
		},
	}

	updates := PrepareUpdates(ganttData, issues, nil)

	if len(updates) != 1 {
		t.Fatalf("expected 1 update for new dates, got %d", len(updates))
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

func TestDetectAtRiskIssues_expected_completion_after_due_date_is_at_risk(t *testing.T) {
	dueDate := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	expectedCompletion := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)

	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 1,
			Title:    "Task with due date",
			DueDate:  &dueDate,
		},
	}

	updates := []DateUpdate{
		{
			Owner:              "owner",
			Repo:               "repo",
			IssueNum:           1,
			ExpectedCompletion: expectedCompletion,
		},
	}

	atRiskIssues := DetectAtRiskIssues(updates, issues)

	if len(atRiskIssues) != 1 {
		t.Fatalf("expected 1 at-risk issue, got %d", len(atRiskIssues))
	}

	if atRiskIssues[0].Reason != "at_risk" {
		t.Errorf("expected reason 'at_risk', got %q", atRiskIssues[0].Reason)
	}
	if atRiskIssues[0].IssueNum != 1 {
		t.Errorf("expected IssueNum 1, got %d", atRiskIssues[0].IssueNum)
	}
	if len(atRiskIssues[0].Details) != 2 {
		t.Fatalf("expected 2 details, got %d", len(atRiskIssues[0].Details))
	}
	if atRiskIssues[0].Details[0] != "Due Date: 2025-03-01" {
		t.Errorf("expected first detail to be due date, got %q", atRiskIssues[0].Details[0])
	}
	if atRiskIssues[0].Details[1] != "Expected Completion: 2025-03-15" {
		t.Errorf("expected second detail to be expected completion, got %q", atRiskIssues[0].Details[1])
	}
}

func TestDetectAtRiskIssues_expected_completion_before_due_date_is_not_at_risk(t *testing.T) {
	dueDate := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
	expectedCompletion := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)

	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 1,
			Title:    "Task with due date",
			DueDate:  &dueDate,
		},
	}

	updates := []DateUpdate{
		{
			Owner:              "owner",
			Repo:               "repo",
			IssueNum:           1,
			ExpectedCompletion: expectedCompletion,
		},
	}

	atRiskIssues := DetectAtRiskIssues(updates, issues)

	if len(atRiskIssues) != 0 {
		t.Fatalf("expected 0 at-risk issues, got %d", len(atRiskIssues))
	}
}

func TestDetectAtRiskIssues_no_due_date_is_not_at_risk(t *testing.T) {
	expectedCompletion := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)

	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 1,
			Title:    "Task without due date",
			DueDate:  nil,
		},
	}

	updates := []DateUpdate{
		{
			Owner:              "owner",
			Repo:               "repo",
			IssueNum:           1,
			ExpectedCompletion: expectedCompletion,
		},
	}

	atRiskIssues := DetectAtRiskIssues(updates, issues)

	if len(atRiskIssues) != 0 {
		t.Fatalf("expected 0 at-risk issues, got %d", len(atRiskIssues))
	}
}

func TestDetectAtRiskIssues_cleared_dates_not_at_risk(t *testing.T) {
	dueDate := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)

	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 1,
			Title:    "On-hold task",
			DueDate:  &dueDate,
		},
	}

	updates := []DateUpdate{
		{
			Owner:       "owner",
			Repo:        "repo",
			IssueNum:    1,
			ClearDates:  true,
			ClearReason: "on hold",
		},
	}

	atRiskIssues := DetectAtRiskIssues(updates, issues)

	if len(atRiskIssues) != 0 {
		t.Fatalf("expected 0 at-risk issues for cleared dates, got %d", len(atRiskIssues))
	}
}

func TestDetectAtRiskIssues_expected_completion_equal_to_due_date_is_not_at_risk(t *testing.T) {
	dueDate := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)
	expectedCompletion := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)

	issues := map[string]IssueWithProject{
		"github.com/owner/repo/issues/1": {
			Owner:    "owner",
			Repo:     "repo",
			IssueNum: 1,
			Title:    "Task with due date",
			DueDate:  &dueDate,
		},
	}

	updates := []DateUpdate{
		{
			Owner:              "owner",
			Repo:               "repo",
			IssueNum:           1,
			ExpectedCompletion: expectedCompletion,
		},
	}

	atRiskIssues := DetectAtRiskIssues(updates, issues)

	if len(atRiskIssues) != 0 {
		t.Fatalf("expected 0 at-risk issues when completion equals due date, got %d", len(atRiskIssues))
	}
}
