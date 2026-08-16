package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hilather/go-lab-dns/internal/releasecontract"
)

type fixtureFile struct {
	CheckRuns []releasecontract.CheckRun `json:"checkRuns"`
}

func loadCheckRuns(fixturePath string) ([]releasecontract.CheckRun, error) {
	if fixturePath != "" {
		raw, err := os.ReadFile(fixturePath)
		if err != nil {
			return nil, fmt.Errorf("read ci fixture: %w", err)
		}
		var doc fixtureFile
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("parse ci fixture: %w", err)
		}
		return doc.CheckRuns, nil
	}
	token := os.Getenv("GH_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	repo := os.Getenv("GITHUB_REPOSITORY")
	sha := os.Getenv("GITHUB_SHA")
	if sha == "" {
		return nil, fmt.Errorf("GITHUB_SHA is required for -require-ci (or pass -ci-fixture)")
	}
	if repo == "" {
		return nil, fmt.Errorf("GITHUB_REPOSITORY is required for -require-ci (or pass -ci-fixture)")
	}
	if token == "" {
		return nil, fmt.Errorf("GH_TOKEN or GITHUB_TOKEN is required for -require-ci (or pass -ci-fixture)")
	}
	return fetchGitHubChecks(token, repo, sha)
}

type ghCheckRuns struct {
	CheckRuns []struct {
		Name       string `json:"name"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
		HeadSHA    string `json:"head_sha"`
	} `json:"check_runs"`
}

func fetchGitHubChecks(token, repo, sha string) ([]releasecontract.CheckRun, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/commits/%s/check-runs?per_page=100", repo, sha)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github check-runs: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var doc ghCheckRuns
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	out := make([]releasecontract.CheckRun, 0, len(doc.CheckRuns))
	for _, r := range doc.CheckRuns {
		out = append(out, releasecontract.CheckRun{
			Name:       r.Name,
			Status:     r.Status,
			Conclusion: r.Conclusion,
			HeadSHA:    r.HeadSHA,
		})
	}
	return out, nil
}
