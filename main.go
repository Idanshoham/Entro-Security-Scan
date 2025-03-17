package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v53/github"
	"golang.org/x/oauth2"
)

var awsKeyPattern = regexp.MustCompile(`(?i)AKIA[0-9A-Z]{16}`)

func main() {
	http.HandleFunc("/scan", handleScan)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server running on port %s...", port)
	http.ListenAndServe(":"+port, nil)
}

func handleScan(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")
	repo := r.URL.Query().Get("repo")
	if owner == "" || repo == "" {
		http.Error(w, "Missing 'owner' or 'repo' query parameters", http.StatusBadRequest)
		return
	}

	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		http.Error(w, "Missing GITHUB_TOKEN environment variable", http.StatusInternalServerError)
		return
	}

	ctx := context.Background()
	client := newGitHubClient(ctx, githubToken)

	branches, _, err := client.Repositories.ListBranches(ctx, owner, repo, nil)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching branches: %v", err), http.StatusInternalServerError)
		return
	}

	results := []map[string]string{}

	for _, branch := range branches {
		branchName := branch.GetName()
		log.Printf("Scanning branch: %s\n", branchName)

		opt := &github.CommitsListOptions{
			SHA:         branchName,
			ListOptions: github.ListOptions{PerPage: 100},
		}

		for {
			commits, resp, err := fetchCommitsWithRetry(client, ctx, owner, repo, opt)
			if err != nil {
				http.Error(w, fmt.Sprintf("Error fetching commits: %v", err), http.StatusInternalServerError)
				return
			}

			if len(commits) == 0 {
				break
			}

			for _, commit := range commits {
				sha := commit.GetSHA()
				log.Printf("Checking commit: %s in branch: %s\n", sha, branchName)

				commitData, err := fetchCommitWithRetry(client, ctx, owner, repo, sha)
				if err != nil {
					log.Printf("Error fetching commit details for %s: %v\n", sha, err)
					continue
				}

				for _, file := range commitData.Files {
					if awsKeyPattern.MatchString(file.GetPatch()) {
						log.Printf("Potential AWS Secret Found in commit %s\n", sha)
						result := map[string]string{
							"commit": sha,
							"branch": branchName,
							"file":   file.GetFilename(),
							"patch":  strings.TrimSpace(file.GetPatch()),
						}
						results = append(results, result)
					}
				}
			}

			if resp.NextPage == 0 {
				break
			}
			opt.Page = resp.NextPage
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// Function to create a GitHub client with TLS verification disabled
func newGitHubClient(ctx context.Context, token string) *github.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Ignore SSL errors
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	oauthClient := oauth2.NewClient(ctx, ts)
	oauthClient.Transport = tr

	return github.NewClient(oauthClient)
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
