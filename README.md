# p2-github-scheduler

A CLI tool and GitHub Action that reschedules GitHub Issues using P2's scheduling algorithm and updates calculated date fields in GitHub Projects.

## Features

- Fetches all issues from a GitHub repository
- Extracts custom field values (Low Estimate, High Estimate) from GitHub Projects
- Runs P2's scheduling algorithm
- Updates calculated date fields in GitHub Projects:
  - Expected Start
  - Expected Completion
  - 98% Completion

## CLI Usage

```bash
# Schedule issues from a GitHub Project
p2-github-scheduler https://github.com/orgs/myorg/projects/1

# Schedule issues from a repository
p2-github-scheduler https://github.com/owner/repo

# Short form for repositories
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

The CLI supports two authentication methods:

1. **Environment variable (CI)**: Set `GITHUB_TOKEN` environment variable
2. **Device Flow (interactive)**: On first run, you'll be prompted to visit a URL and enter a code. The token is stored securely in your system keyring.

## GitHub Actions

The GitHub Action automatically reschedules issues when they are updated. It uses OIDC authentication via p2-penny-pusher to obtain scoped tokens without storing secrets.

### Automatic Setup

1. Install the [p2 GitHub App](https://github.com/apps/october-swimmer-p2) on your repositories
2. A pull request will automatically be created in each repository to add the workflow file
3. Merge the PRs to enable automatic scheduling

### Manual Setup

If you prefer to set up manually, create this workflow file:

```yaml
# .github/workflows/schedule-issues.yml
name: Schedule Issues

on:
  issues:
    types: [opened, edited, closed, reopened, assigned, unassigned, labeled, unlabeled]

permissions:
  id-token: write
  contents: read

jobs:
  schedule:
    runs-on: ubuntu-latest
    steps:
      - name: Schedule project
        uses: octoberswimmer/p2-github-scheduler@main
        with:
          token-broker-url: https://penny-pusher.octoberswimmer.com
```

### Inputs

| Input | Required | Description |
|-------|----------|-------------|
| `token-broker-url` | Yes | URL of the p2-penny-pusher token broker |
| `project-url` | No | GitHub Project URL (auto-detected from issue if not provided) |
| `dry-run` | No | Show changes without applying (default: false) |
| `version` | No | Release tag to install (default: latest) |

### How the Action Works

1. Requests an OIDC token from GitHub Actions
2. Exchanges the OIDC token for a scoped GitHub App installation token via p2-penny-pusher
3. Detects which GitHub Project the issue belongs to
4. Runs the scheduler to recalculate dates
5. Updates the project custom fields

The installation token is scoped to only:
- The repository that triggered the workflow
- Permissions: `issues:read`, `metadata:read`, `projects:write`

## How Scheduling Works

1. Fetches all open issues from the repository
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
