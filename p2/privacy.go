package p2

import (
	"fmt"
	"strconv"
	"strings"
)

// PrivacyFilter redacts information about private repositories that are not the
// current repository. This prevents leaking private repo names, issue titles,
// and dependency references in CI logs.
type PrivacyFilter struct {
	currentRepo  string
	privateRepos map[string]bool
}

// NewPrivacyFilter builds a PrivacyFilter from the current repo and issue data.
// currentRepo should be in "owner/repo" format (e.g. from GITHUB_REPOSITORY env var).
func NewPrivacyFilter(currentRepo string, issues map[string]IssueWithProject) *PrivacyFilter {
	privateRepos := make(map[string]bool)
	for _, iwp := range issues {
		if iwp.IsPrivate {
			key := fmt.Sprintf("%s/%s", iwp.Owner, iwp.Repo)
			privateRepos[key] = true
		}
	}
	return &PrivacyFilter{
		currentRepo:  currentRepo,
		privateRepos: privateRepos,
	}
}

// ShouldRedact returns true if the repo is private and not the current repo.
func (pf *PrivacyFilter) ShouldRedact(owner, repo string) bool {
	key := fmt.Sprintf("%s/%s", owner, repo)
	return pf.privateRepos[key] && key != pf.currentRepo
}

// RedactRepo returns "[private]" if the repo should be redacted, otherwise "owner/repo".
func (pf *PrivacyFilter) RedactRepo(owner, repo string) string {
	if pf.ShouldRedact(owner, repo) {
		return "[private]"
	}
	return fmt.Sprintf("%s/%s", owner, repo)
}

// RedactRef returns "[private] #N" if the repo should be redacted, otherwise "owner/repo #N".
func (pf *PrivacyFilter) RedactRef(owner, repo string, issueNum int) string {
	if pf.ShouldRedact(owner, repo) {
		return fmt.Sprintf("[private] #%d", issueNum)
	}
	return fmt.Sprintf("%s/%s #%d", owner, repo, issueNum)
}

// RedactTitle returns "" if the repo should be redacted, otherwise the title.
func (pf *PrivacyFilter) RedactTitle(owner, repo, title string) string {
	if pf.ShouldRedact(owner, repo) {
		return ""
	}
	return title
}

// RedactSchedulingIssue returns a copy of the SchedulingIssue with redacted details.
func (pf *PrivacyFilter) RedactSchedulingIssue(si SchedulingIssue) SchedulingIssue {
	redacted := si
	redacted.Details = make([]string, len(si.Details))
	for i, detail := range si.Details {
		redacted.Details[i] = pf.RedactDepID(detail)
	}
	return redacted
}

// RedactDepID redacts a dependency ID in "owner/repo#N" format.
func (pf *PrivacyFilter) RedactDepID(depID string) string {
	// Parse "owner/repo#N"
	hashIdx := strings.LastIndex(depID, "#")
	if hashIdx < 0 {
		return depID
	}
	num, err := strconv.Atoi(depID[hashIdx+1:])
	if err != nil {
		return depID
	}
	ownerRepo := depID[:hashIdx]
	slashIdx := strings.Index(ownerRepo, "/")
	if slashIdx < 0 {
		return depID
	}
	owner := ownerRepo[:slashIdx]
	repo := ownerRepo[slashIdx+1:]
	if pf.ShouldRedact(owner, repo) {
		return fmt.Sprintf("[private]#%d", num)
	}
	return depID
}
