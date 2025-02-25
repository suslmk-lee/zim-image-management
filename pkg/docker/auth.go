package docker

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// GetDockerHubToken Docker Hub 토큰을 획득
func GetDockerHubToken(auth DockerHubAuth) (string, error) {
	client := &http.Client{}

	// 1. 토큰 획득
	authURL := "https://auth.docker.io/token?service=registry.docker.io&scope=repository:ratelimitpreview/test:pull"
	req, err := http.NewRequest("GET", authURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %v", err)
	}

	// 인증 정보가 있는 경우에만 Basic 인증 사용
	if auth.Username != "" && auth.Password != "" {
		req.SetBasicAuth(auth.Username, auth.Password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get auth token: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("auth request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %v", err)
	}

	return tokenResp.Token, nil
}
