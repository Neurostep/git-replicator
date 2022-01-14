package main

import (
	"context"
	"fmt"
	"github.com/google/go-github/v41/github"
	"golang.org/x/oauth2"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultGithubTimeout = time.Second * 10

	githubHost      = "github.com"
	githubPullsPath = "pull"
)

type (
	Github struct {
		client *github.Client
	}
	PullRequestData struct {
		BranchName string
		RepoName   string
		RepoURL    string
		Commits    int
	}
)

func NewGithub(token string) *Github {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)

	client := github.NewClient(tc)

	return &Github{
		client: client,
	}
}

func (g *Github) GetPullRequestData(prURL string) (*PullRequestData, error) {
	urlParts, err := url.Parse(prURL)
	if err != nil {
		return nil, err
	}

	pathParts := strings.Split(urlParts.Path, "/")
	pullNumberString := pathParts[len(pathParts)-1]

	pullNumber, err := strconv.Atoi(pullNumberString)
	if err != nil {
		return nil, fmt.Errorf("pull request number is not numeric: %s", pullNumberString)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultGithubTimeout)
	defer cancel()

	pr, _, err := g.client.PullRequests.Get(ctx, pathParts[1], pathParts[2], pullNumber)
	if err != nil {
		return nil, err
	}

	prBranch := pr.GetHead()

	return &PullRequestData{
		Commits:    pr.GetCommits(),
		RepoName:   prBranch.GetRepo().GetName(),
		BranchName: strings.ReplaceAll(prBranch.GetRef(), "refs/heads/", ""),
		RepoURL:    fmt.Sprintf("https://%s/%s", githubHost, prBranch.GetRepo().GetFullName()),
	}, nil
}
