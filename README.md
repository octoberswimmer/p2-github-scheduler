# p2-github-scheduler

A CLI tool that reschedules GitHub Issues using P2's scheduling algorithm and updates the calculated date fields in GitHub Projects.

## Features

- Fetches all issues from a GitHub repository
- Extracts custom field values (Low Estimate, High Estimate) from GitHub Projects
- Runs P2's scheduling algorithm
- Updates calculated date fields in GitHub Projects:
  - Expected Start
  - Expected Completion
  - 98% Completion

## Usage

```bash
# Schedule issues for a repository
p2-github-scheduler https://github.com/owner/repo

# Short form
p2-github-scheduler owner/repo

# Dry run (show changes without updating)
p2-github-scheduler --dry-run owner/repo

# Enable debug logging
p2-github-scheduler --debug owner/repo
```

## GitHub Project Setup

For the scheduler to update dates, your GitHub Project needs the following custom fields:

| Field Name | Type | Description |
|------------|------|-------------|
| Low Estimate | Number | Low estimate in hours (read) |
| High Estimate | Number | High estimate in hours (read) |
| Scheduling Status | Single select | Set to "On Hold" to exclude from scheduling (read) |
| Expected Start | Date | Calculated start date (written) |
| Expected Completion | Date | Mean completion date (written) |
| 98% Completion | Date | 98th percentile completion date (written) |

Tasks with Scheduling Status set to "On Hold" will have their date fields cleared.

## Authentication

The tool uses GitHub's Device Flow for authentication. On first run, you'll be prompted to visit a URL and enter a code. The token is stored securely in your system keyring.

## How It Works

1. Fetches all issues from the specified repository
2. Identifies which issues are linked to GitHub Projects
3. Extracts estimate values from Project custom fields
4. Converts issues to P2 tasks with:
   - Issue number as task ID
   - Issue title as task name
   - Assignee as task user
   - Milestone as package
   - Closed state as done
5. Runs P2's scheduler with statistical analysis for completion date ranges
6. Updates the calculated date fields in GitHub Projects
