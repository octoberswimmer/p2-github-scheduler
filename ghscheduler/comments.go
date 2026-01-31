package ghscheduler

import (
	"fmt"
	"strings"

	"github.com/octoberswimmer/p2/github"
)

// SchedulingCommentMarker is the HTML comment marker used to identify scheduler comments
const SchedulingCommentMarker = "<!-- p2-scheduler-comment -->"

// FormatSchedulingComment creates a comment body for a scheduling issue
func FormatSchedulingComment(si github.SchedulingIssue) string {
	var sb strings.Builder
	sb.WriteString(SchedulingCommentMarker)
	sb.WriteString("\n**Scheduling Notice**\n\n")

	switch si.Reason {
	case "cycle":
		sb.WriteString("This issue cannot be scheduled because it is part of a dependency cycle.\n\n")
		sb.WriteString("**Cycle path:**\n")
		for i, id := range si.Details {
			if i > 0 {
				sb.WriteString(" → ")
			}
			sb.WriteString(id)
		}
		sb.WriteString("\n")
	case "missing_dependency":
		sb.WriteString("This issue cannot be scheduled because it depends on issues not in this project.\n\n")
		sb.WriteString("**Missing dependencies:**\n")
		for _, dep := range si.Details {
			sb.WriteString(fmt.Sprintf("- %s\n", dep))
		}
	case "onhold_dependency":
		sb.WriteString("This issue cannot be scheduled because it depends on issues that are on hold.\n\n")
		sb.WriteString("**On-hold dependencies:**\n")
		for _, dep := range si.Details {
			sb.WriteString(fmt.Sprintf("- %s\n", dep))
		}
	case "inaccessible_dependency":
		sb.WriteString("This issue cannot be scheduled because it depends on issues from repositories the p2 GitHub App cannot access.\n\n")
		sb.WriteString("**Details:**\n")
		for _, detail := range si.Details {
			sb.WriteString(fmt.Sprintf("- %s\n", detail))
		}
		sb.WriteString("\nGrant the p2 GitHub App access to all repositories linked in this project.")
	case "missing_estimate":
		sb.WriteString("This issue cannot be scheduled because it is missing required estimate fields.\n\n")
		sb.WriteString("**Missing fields:**\n")
		for _, field := range si.Details {
			sb.WriteString(fmt.Sprintf("- %s\n", field))
		}
	case "invalid_estimate":
		sb.WriteString("This issue cannot be scheduled because the estimates are invalid.\n\n")
		for _, detail := range si.Details {
			sb.WriteString(fmt.Sprintf("%s\n", detail))
		}
	}

	sb.WriteString("\n---\n*This comment is automatically managed by p2-github-scheduler*")
	return sb.String()
}

// FindSchedulingComment finds the comment ID of an existing scheduling comment on an issue
func FindSchedulingComment(client *github.Client, issueNum int) (int64, error) {
	comments, err := client.GetIssueComments(issueNum)
	if err != nil {
		return 0, err
	}

	for _, comment := range comments {
		body, ok := comment["body"].(string)
		if !ok {
			continue
		}
		if strings.HasPrefix(body, SchedulingCommentMarker) {
			// Extract the comment ID
			id, ok := comment["id"].(float64)
			if ok {
				return int64(id), nil
			}
		}
	}

	return 0, nil // No existing comment found
}

// PostOrUpdateSchedulingComment posts a new comment or updates an existing one
func PostOrUpdateSchedulingComment(client *github.Client, si github.SchedulingIssue) error {
	existingID, err := FindSchedulingComment(client, si.IssueNum)
	if err != nil {
		return fmt.Errorf("failed to check for existing comment: %w", err)
	}

	body := FormatSchedulingComment(si)

	if existingID > 0 {
		// Update existing comment
		return client.UpdateIssueComment(existingID, body)
	}

	// Create new comment
	return client.CreateIssueComment(si.IssueNum, body)
}

// DeleteSchedulingComment removes an existing scheduling comment if present
func DeleteSchedulingComment(client *github.Client, issueNum int) error {
	existingID, err := FindSchedulingComment(client, issueNum)
	if err != nil {
		return fmt.Errorf("failed to check for existing comment: %w", err)
	}

	if existingID == 0 {
		return nil // No comment to delete
	}

	return client.DeleteIssueComment(existingID)
}
