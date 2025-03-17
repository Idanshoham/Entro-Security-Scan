package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/go-github/v53/github"
)

func TestAWSKeyDetection(t *testing.T) {
	tests := []struct {
		patch  string
		expect bool
	}{
		{"Some random text", false},
		{"AKIAEXAMPLEKEY1234567", true},
		{"Secret Key: AKIA1234AWSSECRETKEY", true},
		{"No AWS key here", false},
		{"AWS Key: AKIA111111111111111111", true},
		{"This should not match: AKIA12345", false},
	}

	for _, test := range tests {
		result := awsKeyPattern.MatchString(test.patch)
		if result != test.expect {
			t.Errorf("Expected %v, but got %v for patch: %s", test.expect, result, test.patch)
		}
	}
}

func TestPaginationHandling(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/someowner/somerepo/commits" {
			w.Header().Set("Link", `<https://api.github.com/repositories/12345/commits?page=2>; rel="next"`)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"sha": "abc123"}]`))
		} else if r.URL.Path == "/repositories/12345/commits" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[{"sha": "def456"}]`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	ctx := context.Background()
	client := github.NewClient(&http.Client{})

	opt := &github.CommitsListOptions{ListOptions: github.ListOptions{PerPage: 1}}
	pageCount := 0

	for {
		_, resp, err := client.Repositories.ListCommits(ctx, "Idanshoham", "Currency-Converter-Tool-Java", opt)
		if err != nil {
			t.Fatalf("Error fetching commits: %v", err)
		}

		pageCount++
		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	if pageCount < 2 {
		t.Errorf("Expected pagination to fetch multiple pages, got %d pages", pageCount)
	}
}

func TestLastCommitStorage(t *testing.T) {
	sha := "testcommit1234"
	saveLastCommit(sha)
	savedSha := getLastCommit()

	if savedSha != sha {
		t.Errorf("Expected %s, got %s", sha, savedSha)
	}

	// Cleanup
	_ = os.Remove("last_commit.txt")
}
