package github

import (
	"context"
	"errors"
)

// Client wraps the GitHub API for FORGE operations.
//
// TODO(forge): implement per docs/platform-plan.jsx Week 9
// Uses GITHUB_TOKEN env var for authentication.
type Client struct {
	// Token is the GitHub personal access token or GitHub App installation token.
	Token string
}

// CreatePR opens a pull request for the given branch with title and body.
// Returns the PR number on success.
// TODO(forge): implement per docs/platform-plan.jsx Week 9
func (c *Client) CreatePR(_ context.Context, branch, title, body string) (int, error) {
	return 0, errors.New("github client: not implemented")
}

// GetReviewComments fetches all review comments on the given PR number.
// TODO(forge): implement per docs/platform-plan.jsx Week 9
func (c *Client) GetReviewComments(_ context.Context, prNumber int) ([]ReviewComment, error) {
	return nil, errors.New("github client: not implemented")
}

// UpdatePR updates the title and/or body of an existing PR.
// TODO(forge): implement per docs/platform-plan.jsx Week 9
func (c *Client) UpdatePR(_ context.Context, prNumber int, title, body string) error {
	return errors.New("github client: not implemented")
}

// Merge squash-merges the pull request identified by prNumber.
// TODO(forge): implement per docs/platform-plan.jsx Week 9
func (c *Client) Merge(_ context.Context, prNumber int) error {
	return errors.New("github client: not implemented")
}

// ReviewComment is a single code-review comment on a pull request.
type ReviewComment struct {
	ID   int64
	Body string
	Path string
	Line int
}
