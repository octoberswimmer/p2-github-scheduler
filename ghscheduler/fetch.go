package ghscheduler

import (
	"fmt"

	"github.com/octoberswimmer/p2-github-scheduler/p2"
	"github.com/octoberswimmer/p2/github"
	"github.com/sirupsen/logrus"
)

// FetchProjectItems fetches all items from a GitHub Project
func FetchProjectItems(accessToken string, info *URLInfo) (map[string]p2.IssueWithProject, error) {
	project := &github.GitHubProject{
		Owner:  info.Owner,
		Number: info.ProjectNum,
		IsOrg:  info.IsOrg,
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

	allIssues := make(map[string]p2.IssueWithProject)
	var inaccessibleItems []string

	// Track position for ordering (GraphQL returns items in display order)
	position := 0
	for _, item := range items {
		var ref string
		var isDraft bool

		if item.ContentType == "ISSUE" {
			ref = fmt.Sprintf("github.com/%s/%s/issues/%d", item.RepoOwner, item.RepoName, item.IssueNumber)
		} else if item.ContentType == "DRAFT_ISSUE" {
			// Draft issues use project item ID as ref and are treated as on-hold
			ref = fmt.Sprintf("draft:%s", item.ID)
			isDraft = true
			logrus.Debugf("Including draft issue as on-hold: %s", item.Title)
		} else if item.Title != "" {
			// Item has a title but no content - likely from an inaccessible repo
			inaccessibleItems = append(inaccessibleItems, item.Title)
			continue
		} else {
			// Skip items with no useful content
			continue
		}

		// Extract estimates and status from custom fields
		var lowEstimate, highEstimate float64
		var schedulingStatus string
		var hasEstimates, hasSchedulingDates bool
		if v, ok := item.CustomFields["Low Estimate"].(float64); ok {
			lowEstimate = v
			hasEstimates = true
		}
		if v, ok := item.CustomFields["High Estimate"].(float64); ok {
			highEstimate = v
			hasEstimates = true
		}
		if v, ok := item.CustomFields["Scheduling Status"].(string); ok {
			schedulingStatus = v
		}
		// Check if any scheduling date fields are set
		if _, ok := item.CustomFields["Expected Start"].(string); ok {
			hasSchedulingDates = true
		}
		if _, ok := item.CustomFields["Expected Completion"].(string); ok {
			hasSchedulingDates = true
		}
		if _, ok := item.CustomFields["98% Completion"].(string); ok {
			hasSchedulingDates = true
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

		allIssues[ref] = p2.IssueWithProject{
			Owner:                item.RepoOwner,
			Repo:                 item.RepoName,
			IssueNum:             item.IssueNumber,
			Title:                item.Title,
			Body:                 item.Body,
			State:                item.State,
			Assignee:             assignee,
			Labels:               item.Labels,
			Milestone:            item.Milestone,
			Project:              projectInfo,
			LowEstimate:          lowEstimate,
			HighEstimate:         highEstimate,
			Order:                position,
			SchedulingStatus:     schedulingStatus,
			HasSchedulingDates:   hasSchedulingDates,
			HasEstimates:         hasEstimates,
			BlockedBy:            item.BlockedBy,
			Blocking:             item.Blocking,
			InaccessibleBlockers: item.InaccessibleBlockers,
			IsDraft:              isDraft,
			ProjectItemID:        item.ID,
		}

		var blockedByRefs []string
		for _, b := range item.BlockedBy {
			blockedByRefs = append(blockedByRefs, fmt.Sprintf("%s/%s#%d", b.Owner, b.Repo, b.Number))
		}
		var blockingRefs []string
		for _, b := range item.Blocking {
			blockingRefs = append(blockingRefs, fmt.Sprintf("%s/%s#%d", b.Owner, b.Repo, b.Number))
		}
		logrus.Debugf("Issue %s: low=%.1f, high=%.1f, order=%d, schedulingStatus=%s, hasDates=%v, hasEstimates=%v, blockedBy=%v, blocking=%v, inaccessibleBlockers=%d",
			ref, lowEstimate, highEstimate, position, schedulingStatus, hasSchedulingDates, hasEstimates, blockedByRefs, blockingRefs, item.InaccessibleBlockers)
		position++
	}

	BuildReverseDependencies(allIssues)

	if len(inaccessibleItems) > 0 {
		fmt.Printf("\nSkipped %d inaccessible items:\n", len(inaccessibleItems))
		for _, title := range inaccessibleItems {
			fmt.Printf("  - %s\n", title)
		}
		fmt.Println("Grant the p2 GitHub App access to all repositories linked in this project.")
	}

	return allIssues, nil
}

// FetchRepoIssues fetches all issues from a GitHub repository
func FetchRepoIssues(accessToken string, info *URLInfo) (map[string]p2.IssueWithProject, error) {
	client := github.NewClient(accessToken, &github.GitHubRepository{Owner: info.Owner, Name: info.Repo})

	fmt.Println("Fetching issues...")
	issues, err := client.ListIssues("all")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch issues: %w", err)
	}
	fmt.Printf("Found %d issues\n", len(issues))

	if len(issues) == 0 {
		return nil, nil
	}

	allIssues := make(map[string]p2.IssueWithProject)

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
		schedulingStatus := ExtractSchedulingStatus(issueData)

		ref := fmt.Sprintf("github.com/%s/%s/issues/%d", info.Owner, info.Repo, issue.Number)

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

		allIssues[ref] = p2.IssueWithProject{
			Owner:            info.Owner,
			Repo:             info.Repo,
			IssueNum:         issue.Number,
			Title:            issue.Title,
			Body:             issue.Body,
			State:            issue.State,
			Assignee:         assignee,
			Labels:           labels,
			Milestone:        milestone,
			Project:          projectInfo,
			LowEstimate:      lowEstimate,
			HighEstimate:     highEstimate,
			Order:            order,
			SchedulingStatus: schedulingStatus,
		}

		logrus.Debugf("Issue #%d is in project %s (low=%.1f, high=%.1f, order=%d, schedulingStatus=%s)",
			issue.Number, projectInfo.ProjectID, lowEstimate, highEstimate, order, schedulingStatus)
	}

	return allIssues, nil
}

// ExtractSchedulingStatus extracts the "Scheduling Status" single-select field value from issue data
func ExtractSchedulingStatus(issueData map[string]interface{}) string {
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

// BuildReverseDependencies adds blockedBy entries based on blocking relationships.
// If issue A is blocking issue B, then B's blockedBy should include A.
func BuildReverseDependencies(issues map[string]p2.IssueWithProject) {
	for ref, iwp := range issues {
		for _, blocked := range iwp.Blocking {
			blockedRef := fmt.Sprintf("github.com/%s/%s/issues/%d", blocked.Owner, blocked.Repo, blocked.Number)
			if blockedIssue, ok := issues[blockedRef]; ok {
				// Check if dependency already exists
				alreadyExists := false
				for _, existing := range blockedIssue.BlockedBy {
					if existing.Owner == iwp.Owner && existing.Repo == iwp.Repo && existing.Number == iwp.IssueNum {
						alreadyExists = true
						break
					}
				}
				if !alreadyExists {
					blockedIssue.BlockedBy = append(blockedIssue.BlockedBy, github.IssueRef{
						Owner:  iwp.Owner,
						Repo:   iwp.Repo,
						Number: iwp.IssueNum,
					})
					issues[blockedRef] = blockedIssue
					logrus.Debugf("Added reverse dependency: %s blocked by %s (from blocking field)", blockedRef, ref)
				}
			}
		}
	}
}
