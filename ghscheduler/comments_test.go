package ghscheduler

import (
	"strings"
	"testing"

	"github.com/octoberswimmer/p2-github-scheduler/p2"
)

func TestFormatSchedulingComment_Cycle(t *testing.T) {
	si := p2.SchedulingIssue{
		Reason:  "cycle",
		Details: []string{"owner/repo#1", "owner/repo#2", "owner/repo#1"},
	}

	comment := FormatSchedulingComment(si)

	if !strings.Contains(comment, SchedulingCommentMarker) {
		t.Error("comment should contain the marker")
	}
	if !strings.Contains(comment, "dependency cycle") {
		t.Error("comment should mention dependency cycle")
	}
	if !strings.Contains(comment, "owner/repo#1") {
		t.Error("comment should contain cycle path")
	}
}

func TestFormatSchedulingComment_MissingDependency(t *testing.T) {
	si := p2.SchedulingIssue{
		Reason:  "missing_dependency",
		Details: []string{"other/repo#99"},
	}

	comment := FormatSchedulingComment(si)

	if !strings.Contains(comment, SchedulingCommentMarker) {
		t.Error("comment should contain the marker")
	}
	if !strings.Contains(comment, "not in this project") {
		t.Error("comment should mention issues not in project")
	}
	if !strings.Contains(comment, "other/repo#99") {
		t.Error("comment should list the missing dependency")
	}
}

func TestFormatSchedulingComment_OnHoldDependency(t *testing.T) {
	si := p2.SchedulingIssue{
		Reason:  "onhold_dependency",
		Details: []string{"owner/repo#5"},
	}

	comment := FormatSchedulingComment(si)

	if !strings.Contains(comment, SchedulingCommentMarker) {
		t.Error("comment should contain the marker")
	}
	if !strings.Contains(comment, "on hold") {
		t.Error("comment should mention on hold")
	}
	if !strings.Contains(comment, "owner/repo#5") {
		t.Error("comment should list the on-hold dependency")
	}
}

func TestFormatSchedulingComment_MissingEstimate(t *testing.T) {
	si := p2.SchedulingIssue{
		Reason:  "missing_estimate",
		Details: []string{"Low Estimate"},
	}

	comment := FormatSchedulingComment(si)

	if !strings.Contains(comment, SchedulingCommentMarker) {
		t.Error("comment should contain the marker")
	}
	if !strings.Contains(comment, "missing required estimate") {
		t.Error("comment should mention missing estimate")
	}
	if !strings.Contains(comment, "Low Estimate") {
		t.Error("comment should list the missing field")
	}
}

func TestFormatSchedulingComment_InvalidEstimate(t *testing.T) {
	si := p2.SchedulingIssue{
		Reason:  "invalid_estimate",
		Details: []string{"High Estimate (4.0) must be greater than or equal to Low Estimate (8.0)"},
	}

	comment := FormatSchedulingComment(si)

	if !strings.Contains(comment, SchedulingCommentMarker) {
		t.Error("comment should contain the marker")
	}
	if !strings.Contains(comment, "estimates are invalid") {
		t.Error("comment should mention invalid estimates")
	}
	if !strings.Contains(comment, "must be greater than or equal to") {
		t.Error("comment should contain the detail message")
	}
}

func TestFormatSchedulingComment_AtRisk(t *testing.T) {
	si := p2.SchedulingIssue{
		Reason:  "at_risk",
		Details: []string{"Due Date: 2025-03-01", "Expected Completion: 2025-03-15"},
	}

	comment := FormatSchedulingComment(si)

	if !strings.Contains(comment, SchedulingCommentMarker) {
		t.Error("comment should contain the marker")
	}
	if !strings.Contains(comment, "at risk") {
		t.Error("comment should mention at risk")
	}
	if !strings.Contains(comment, "due date") {
		t.Error("comment should mention due date")
	}
	if !strings.Contains(comment, "Due Date: 2025-03-01") {
		t.Error("comment should contain the due date")
	}
	if !strings.Contains(comment, "Expected Completion: 2025-03-15") {
		t.Error("comment should contain the expected completion date")
	}
}
