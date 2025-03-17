package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v53/github"
	"golang.org/x/oauth2"
)

var awsKeyPattern = regexp.MustCompile(`(?i)AKIA[0-9A-Z]{16}`)

func main() {
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		log.Fatal("Missing GITHUB_TOKEN environment variable")
	}

	owner, repo := "Idanshoham", "Currency-Converter-Tool-Java" // Replace with actual values
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubToken})
	client := github.NewClient(oauth2.NewClient(ctx, ts))

	opt := &github.CommitsListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	lastCommit := getLastCommit()
	skipCommits := lastCommit != ""

	for {
		commits, resp, err := fetchCommitsWithRetry(client, ctx, owner, repo, opt)
		if err != nil {
			log.Fatalf("Error fetching commits: %v", err)
		}

		if len(commits) == 0 {
			break
		}

		for _, commit := range commits {
			sha := commit.GetSHA()

			if skipCommits {
				if sha == lastCommit {
					skipCommits = false
				}
				continue
			}

			fmt.Printf("Checking commit: %s\n", sha)

			commitData, err := fetchCommitWithRetry(client, ctx, owner, repo, sha)
			if err != nil {
				log.Printf("Error fetching commit details for %s: %v\n", sha, err)
				continue
			}

			for _, file := range commitData.Files {
				if awsKeyPattern.MatchString(file.GetPatch()) {
					fmt.Printf("Potential AWS Secret Found in commit %s by %s\n", sha, commitData.GetCommit().GetCommitter().GetName())
					fmt.Printf("File: %s\n", file.GetFilename())
					fmt.Printf("Patch: %s\n", strings.TrimSpace(file.GetPatch()))
				}
			}

			saveLastCommit(sha)
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
}

func fetchCommitsWithRetry(client *github.Client, ctx context.Context, owner, repo string, opt *github.CommitsListOptions) ([]*github.RepositoryCommit, *github.Response, error) {
	for {
		commits, resp, err := client.Repositories.ListCommits(ctx, owner, repo, opt)
		if err == nil {
			return commits, resp, nil
		}

		if resp != nil && resp.StatusCode == 403 {
			handleRateLimit(resp)
			continue
		}

		return nil, nil, err
	}
}

func saveLastCommit(sha string) {
	err := os.WriteFile("last_commit.txt", []byte(sha), 0644)
	if err != nil {
		log.Printf("Error saving last commit: %v", err)
	}
}

func getLastCommit() string {
	data, err := os.ReadFile("last_commit.txt")
	if err != nil {
		return "" // No previous scan found, start from the beginning
	}
	return strings.TrimSpace(string(data))
}

func fetchCommitWithRetry(client *github.Client, ctx context.Context, owner, repo, sha string) (*github.RepositoryCommit, error) {
	for {
		commit, resp, err := client.Repositories.GetCommit(ctx, owner, repo, sha, nil)
		if err == nil {
			return commit, nil
		}

		if resp != nil && resp.StatusCode == 403 {
			handleRateLimit(resp)
			continue
		}

		return nil, err
	}
}

func handleRateLimit(resp *github.Response) {
	resetTime := time.Now().Add(60 * time.Second) // Default to 60 seconds if unknown

	if resp.Rate.Limit > 0 {
		resetTime = resp.Rate.Reset.Time
	}

	log.Printf("Rate limit exceeded. Waiting until %v...\n", resetTime)
	time.Sleep(time.Until(resetTime))
}
