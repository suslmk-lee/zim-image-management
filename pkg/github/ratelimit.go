package github

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// GetDockerRateLimit GitHub Container Registry의 rate limit 정보를 조회
func GetDockerRateLimit(token string) (*RateLimit, error) {
	if token == "" {
		return nil, fmt.Errorf("GitHub token is required")
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.github.com/rate_limit", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	var rateLimit struct {
		Resources struct {
			Core RateLimit `json:"core"`
		} `json:"resources"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rateLimit); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return &rateLimit.Resources.Core, nil
}

// PrintGitHubRateLimit GitHub Container Registry rate limit 정보를 출력
func PrintGitHubRateLimit(rateLimit *RateLimit) {
	fmt.Printf("\nGitHub Docker Rate Limits:\n")
	fmt.Printf("================================\n")
	fmt.Printf("Limit: %d\n", rateLimit.Limit)
	fmt.Printf("Remaining: %d\n", rateLimit.Remaining)
	fmt.Printf("Used: %d\n", rateLimit.Used)
	fmt.Printf("Reset Time: %s\n", rateLimit.GetResetTime().Format("2006-01-02 15:04:05"))
	fmt.Printf("================================\n\n")
}
