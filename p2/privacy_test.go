package p2

import (
	"testing"
)

func newTestFilter() *PrivacyFilter {
	issues := map[string]IssueWithProject{
		"github.com/myorg/myrepo/issues/1": {
			Owner:     "myorg",
			Repo:      "myrepo",
			IssueNum:  1,
			Title:     "Current repo issue",
			IsPrivate: true,
		},
		"github.com/myorg/secret/issues/2": {
			Owner:     "myorg",
			Repo:      "secret",
			IssueNum:  2,
			Title:     "Secret issue title",
			IsPrivate: true,
		},
		"github.com/myorg/public/issues/3": {
			Owner:    "myorg",
			Repo:     "public",
			IssueNum: 3,
			Title:    "Public issue",
		},
	}
	return NewPrivacyFilter("myorg/myrepo", issues)
}

func TestShouldRedact_current_repo_is_not_redacted(t *testing.T) {
	pf := newTestFilter()
	if pf.ShouldRedact("myorg", "myrepo") {
		t.Error("current repo should not be redacted")
	}
}

func TestShouldRedact_other_private_repo_is_redacted(t *testing.T) {
	pf := newTestFilter()
	if !pf.ShouldRedact("myorg", "secret") {
		t.Error("other private repo should be redacted")
	}
}

func TestShouldRedact_public_repo_is_not_redacted(t *testing.T) {
	pf := newTestFilter()
	if pf.ShouldRedact("myorg", "public") {
		t.Error("public repo should not be redacted")
	}
}

func TestShouldRedact_unknown_repo_is_not_redacted(t *testing.T) {
	pf := newTestFilter()
	if pf.ShouldRedact("other", "unknown") {
		t.Error("unknown repo should not be redacted")
	}
}

func TestRedactRepo_private_repo(t *testing.T) {
	pf := newTestFilter()
	got := pf.RedactRepo("myorg", "secret")
	if got != "[private]" {
		t.Errorf("expected [private], got %q", got)
	}
}

func TestRedactRepo_current_repo(t *testing.T) {
	pf := newTestFilter()
	got := pf.RedactRepo("myorg", "myrepo")
	if got != "myorg/myrepo" {
		t.Errorf("expected myorg/myrepo, got %q", got)
	}
}

func TestRedactRepo_public_repo(t *testing.T) {
	pf := newTestFilter()
	got := pf.RedactRepo("myorg", "public")
	if got != "myorg/public" {
		t.Errorf("expected myorg/public, got %q", got)
	}
}

func TestRedactRef_private_repo(t *testing.T) {
	pf := newTestFilter()
	got := pf.RedactRef("myorg", "secret", 42)
	if got != "[private] #42" {
		t.Errorf("expected [private] #42, got %q", got)
	}
}

func TestRedactRef_current_repo(t *testing.T) {
	pf := newTestFilter()
	got := pf.RedactRef("myorg", "myrepo", 1)
	if got != "myorg/myrepo #1" {
		t.Errorf("expected myorg/myrepo #1, got %q", got)
	}
}

func TestRedactTitle_private_repo(t *testing.T) {
	pf := newTestFilter()
	got := pf.RedactTitle("myorg", "secret", "Secret issue title")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestRedactTitle_current_repo(t *testing.T) {
	pf := newTestFilter()
	got := pf.RedactTitle("myorg", "myrepo", "Current repo issue")
	if got != "Current repo issue" {
		t.Errorf("expected title, got %q", got)
	}
}

func TestRedactDepID_private_repo(t *testing.T) {
	pf := newTestFilter()
	got := pf.RedactDepID("myorg/secret#2")
	if got != "[private]#2" {
		t.Errorf("expected [private]#2, got %q", got)
	}
}

func TestRedactDepID_current_repo(t *testing.T) {
	pf := newTestFilter()
	got := pf.RedactDepID("myorg/myrepo#1")
	if got != "myorg/myrepo#1" {
		t.Errorf("expected myorg/myrepo#1, got %q", got)
	}
}

func TestRedactDepID_public_repo(t *testing.T) {
	pf := newTestFilter()
	got := pf.RedactDepID("myorg/public#3")
	if got != "myorg/public#3" {
		t.Errorf("expected myorg/public#3, got %q", got)
	}
}

func TestRedactDepID_invalid_format(t *testing.T) {
	pf := newTestFilter()
	got := pf.RedactDepID("not-a-dep-id")
	if got != "not-a-dep-id" {
		t.Errorf("expected unchanged string, got %q", got)
	}
}
