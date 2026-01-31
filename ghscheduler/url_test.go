package ghscheduler

import (
	"testing"
)

func TestParseGitHubURL(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantOwner  string
		wantRepo   string
		wantIsOrg  bool
		wantIsPrj  bool
		wantPrjNum int
		wantErr    bool
	}{
		{
			name:      "full_repo_url",
			url:       "https://github.com/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "repo_url_with_trailing_slash",
			url:       "https://github.com/owner/repo/",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "issue_url",
			url:       "https://github.com/owner/repo/issues/123",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "short_form",
			url:       "owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:       "org_project_url",
			url:        "https://github.com/orgs/myorg/projects/1",
			wantOwner:  "myorg",
			wantIsOrg:  true,
			wantIsPrj:  true,
			wantPrjNum: 1,
		},
		{
			name:       "user_project_url",
			url:        "https://github.com/users/myuser/projects/42",
			wantOwner:  "myuser",
			wantIsOrg:  false,
			wantIsPrj:  true,
			wantPrjNum: 42,
		},
		{
			name:      "owner_with_dashes",
			url:       "https://github.com/my-org/my-repo",
			wantOwner: "my-org",
			wantRepo:  "my-repo",
		},
		{
			name:    "invalid_url",
			url:     "not-a-github-url",
			wantErr: true,
		},
		{
			name:    "empty_url",
			url:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ParseGitHubURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseGitHubURL(%q) expected error, got nil", tt.url)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseGitHubURL(%q) unexpected error: %v", tt.url, err)
				return
			}
			if info.Owner != tt.wantOwner {
				t.Errorf("ParseGitHubURL(%q) owner = %q, want %q", tt.url, info.Owner, tt.wantOwner)
			}
			if info.Repo != tt.wantRepo {
				t.Errorf("ParseGitHubURL(%q) repo = %q, want %q", tt.url, info.Repo, tt.wantRepo)
			}
			if info.IsOrg != tt.wantIsOrg {
				t.Errorf("ParseGitHubURL(%q) isOrg = %v, want %v", tt.url, info.IsOrg, tt.wantIsOrg)
			}
			if info.IsProject != tt.wantIsPrj {
				t.Errorf("ParseGitHubURL(%q) isProject = %v, want %v", tt.url, info.IsProject, tt.wantIsPrj)
			}
			if info.ProjectNum != tt.wantPrjNum {
				t.Errorf("ParseGitHubURL(%q) projectNum = %d, want %d", tt.url, info.ProjectNum, tt.wantPrjNum)
			}
		})
	}
}
