# p2-github-scheduler

A GitHub Action that automatically reschedules GitHub Issues using p2's scheduling algorithm and updates calculated date fields in GitHub Projects.

## Quick Start

1. Install the [p2 GitHub App](https://github.com/apps/october-swimmer-p2) on your repositories
2. A pull request will automatically be created to add the workflow file
3. Merge the PR to enable automatic scheduling

That's it! Issues will be automatically rescheduled whenever they are updated.

## GitHub Project Setup

Your GitHub Project needs the following custom fields:

| Field Name | Type | Description |
|------------|------|-------------|
| Low Estimate | Number | Low estimate in hours (read) |
| High Estimate | Number | High estimate in hours (read) |
| Scheduling Status | Single select | Set to "On Hold" to exclude from scheduling (read) |
| Expected Start | Date | Calculated start date (written) |
| Expected Completion | Date | Mean completion date (written) |
| 98% Completion | Date | 98th percentile completion date (written) |

Tasks with Scheduling Status set to "On Hold" will have their date fields cleared.

## Scheduling Warnings

When an issue cannot be scheduled, a comment is automatically posted to the issue explaining the problem. Comments are automatically removed when the issue becomes schedulable.

Issues cannot be scheduled when:

- **Dependency cycle**: The issue is part of a circular dependency chain (A blocks B, B blocks A)
- **Missing dependency**: The issue depends on another issue that is not in the project
- **On-hold dependency**: The issue depends on another issue that has Scheduling Status set to "On Hold"

Issues with scheduling problems will not have their date fields updated until the problem is resolved.

## Manual Workflow Setup

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

concurrency:
  group: p2-schedule-${{ github.repository }}
  cancel-in-progress: true

jobs:
  schedule:
    runs-on: ubuntu-latest
    steps:
      - name: Schedule project
        uses: octoberswimmer/p2-github-scheduler@main
        with:
          token-broker-url: https://penny-pusher.octoberswimmer.com
```

### Workflow Inputs

| Input | Required | Description |
|-------|----------|-------------|
| `token-broker-url` | Yes | URL of the p2-penny-pusher token broker |
| `project-url` | No | GitHub Project URL (auto-detected from issue if not provided) |
| `dry-run` | No | Show changes without applying (default: false) |
| `version` | No | Release tag to install (default: latest) |

## How It Works

1. When an issue is updated, the workflow requests an OIDC token from GitHub Actions
2. The OIDC token is exchanged for a scoped GitHub App installation token via p2-penny-pusher
3. The scheduler detects which GitHub Project the issue belongs to
4. All issues are fetched and converted to p2 tasks with:
   - Issue number as task ID
   - Issue title as task name
   - Assignee as task user
   - Milestone as package
   - GitHub blocking relationships as task dependencies
5. p2's scheduler runs statistical analysis to calculate completion date ranges
6. The calculated date fields are updated in GitHub Projects

The installation token is scoped to only:
- The repository that triggered the workflow
- Permissions: `issues:write`, `metadata:read`, `projects:write`

The `issues:write` permission is required to post and manage scheduling warning comments.

## CLI Usage

A CLI is also available for local testing and debugging:

```bash
# Schedule issues from a GitHub Project
p2-github-scheduler https://github.com/orgs/myorg/projects/1

# Dry run (show changes without updating)
p2-github-scheduler --dry-run owner/repo

# Enable debug logging
p2-github-scheduler --debug owner/repo
```

### CLI Authentication

The CLI supports two authentication methods:

1. **Environment variable**: Set `GITHUB_TOKEN` environment variable
2. **Device Flow (interactive)**: On first run, you'll be prompted to authenticate via browser. The token is stored securely in your system keyring.
