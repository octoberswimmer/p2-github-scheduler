package ghscheduler

import (
	"fmt"

	"github.com/octoberswimmer/p2-github-scheduler/p2"
	"github.com/octoberswimmer/p2/github"
	"github.com/sirupsen/logrus"
)

// ApplyUpdate writes date updates to GitHub
func ApplyUpdate(client *github.Client, update p2.DateUpdate) error {
	if update.Project == nil {
		return fmt.Errorf("no project info")
	}

	// Clear scheduling fields for closed/on-hold tasks
	if update.ClearDates {
		// Always clear date fields
		fieldsToClean := []string{
			"Expected Start",
			"Expected Completion",
			"98% Completion",
		}
		// Only clear estimates if closed (not on hold)
		if update.ClearReason == "closed" {
			fieldsToClean = append(fieldsToClean, "Low Estimate", "High Estimate")
		}
		for _, fieldName := range fieldsToClean {
			if fieldID, ok := update.Project.FieldIDs[fieldName]; ok {
				if err := client.ClearField(update.Project.ProjectID, update.Project.ItemID, fieldID); err != nil {
					logrus.Warnf("Failed to clear %s for #%d: %v", fieldName, update.IssueNum, err)
				}
			}
		}
		return nil
	}

	// Update Expected Start
	if !update.ExpectedStart.IsZero() {
		if fieldID, ok := update.Project.FieldIDs["Expected Start"]; ok {
			if err := client.UpdateDateField(update.Project.ProjectID, update.Project.ItemID, fieldID, update.ExpectedStart); err != nil {
				logrus.Warnf("Failed to update Expected Start for #%d: %v", update.IssueNum, err)
			}
		} else {
			logrus.Debugf("No 'Expected Start' field found for issue #%d", update.IssueNum)
		}
	}

	// Update Expected Completion
	if !update.ExpectedCompletion.IsZero() {
		if fieldID, ok := update.Project.FieldIDs["Expected Completion"]; ok {
			if err := client.UpdateDateField(update.Project.ProjectID, update.Project.ItemID, fieldID, update.ExpectedCompletion); err != nil {
				logrus.Warnf("Failed to update Expected Completion for #%d: %v", update.IssueNum, err)
			}
		} else {
			logrus.Debugf("No 'Expected Completion' field found for issue #%d", update.IssueNum)
		}
	}

	// Update 98% Completion
	if !update.Completion98.IsZero() {
		if fieldID, ok := update.Project.FieldIDs["98% Completion"]; ok {
			if err := client.UpdateDateField(update.Project.ProjectID, update.Project.ItemID, fieldID, update.Completion98); err != nil {
				logrus.Warnf("Failed to update 98%% Completion for #%d: %v", update.IssueNum, err)
			}
		} else {
			logrus.Debugf("No '98%% Completion' field found for issue #%d", update.IssueNum)
		}
	}

	return nil
}
