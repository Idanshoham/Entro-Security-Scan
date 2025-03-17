package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/google/go-github/v53/github"
	"golang.org/x/oauth2"
)

var awsKeyPattern = regexp.MustCompile(`(?i)AKIA[0-9A-Z]{16}`)

func main() {
	// Load GitHub token from env variable
	githubToken := os.Getenv("GITHUB_TOKEN")

	if githubToken == "" {
		log.Fatal("Missing GITHUB_TOKEN environment variable")
	}

	// Define repo owner and name
	owner, repo := "Idanshoham", "Currency-Converter-Tool-Java" // Replace with actual values

	// Authenticate with GitHub API
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: githubToken})
	client := github.NewClient(oauth2.NewClient(ctx, ts))

	// Fetch commits
	commits, _, err := client.Repositories.ListCommits(ctx, owner, repo, nil)
	if err != nil {
		log.Fatalf("Error fetching commits: %v", err)
	}

	for _, commit := range commits {
		sha := commit.GetSHA()
		fmt.Printf("Checking commit: %s\n", sha)

		// Fetch commit details
		commitData, _, err := client.Repositories.GetCommit(ctx, owner, repo, sha, nil)
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
	}
}
