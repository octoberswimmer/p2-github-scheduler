package ghscheduler

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// URLInfo contains parsed information from a GitHub URL
type URLInfo struct {
	Owner      string
	Repo       string
	IsOrg      bool
	IsProject  bool
	ProjectNum int
}

// ParseGitHubURL parses a GitHub URL and returns information about it
func ParseGitHubURL(url string) (*URLInfo, error) {
	// Remove trailing slashes and protocol
	url = strings.TrimSuffix(url, "/")
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Org project URL: github.com/orgs/org/projects/N
	orgProjectRe := regexp.MustCompile(`^github\.com/orgs/([^/]+)/projects/(\d+)`)
	if matches := orgProjectRe.FindStringSubmatch(url); matches != nil {
		num, _ := strconv.Atoi(matches[2])
		return &URLInfo{
			Owner:      matches[1],
			IsOrg:      true,
			IsProject:  true,
			ProjectNum: num,
		}, nil
	}

	// User project URL: github.com/users/user/projects/N
	userProjectRe := regexp.MustCompile(`^github\.com/users/([^/]+)/projects/(\d+)`)
	if matches := userProjectRe.FindStringSubmatch(url); matches != nil {
		num, _ := strconv.Atoi(matches[2])
		return &URLInfo{
			Owner:      matches[1],
			IsOrg:      false,
			IsProject:  true,
			ProjectNum: num,
		}, nil
	}

	// Repo URL: github.com/owner/repo or github.com/owner/repo/issues/N
	repoRe := regexp.MustCompile(`^github\.com/([^/]+)/([^/]+)`)
	if matches := repoRe.FindStringSubmatch(url); matches != nil {
		return &URLInfo{
			Owner: matches[1],
			Repo:  matches[2],
		}, nil
	}

	// Short form: owner/repo
	shortRe := regexp.MustCompile(`^([^/]+)/([^/]+)$`)
	if matches := shortRe.FindStringSubmatch(url); matches != nil {
		return &URLInfo{
			Owner: matches[1],
			Repo:  matches[2],
		}, nil
	}

	return nil, fmt.Errorf("could not parse GitHub URL: %s", url)
}
