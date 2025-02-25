package docker

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// parseRateLimit "200;w=21600" 형식의 문자열에서 숫자 값을 추출
func parseRateLimit(value string) int {
	if value == "" {
		return 0
	}
	// 세미콜론 앞의 숫자만 추출
	parts := strings.Split(value, ";")
	if len(parts) == 0 {
		return 0
	}
	limit, _ := strconv.Atoi(parts[0])
	return limit
}

// GetDockerHubRateLimit Docker Hub의 rate limit 정보를 조회
func GetDockerHubRateLimit(auth DockerHubAuth) (*DockerHubRateLimit, error) {
	client := &http.Client{}

	// 1. 토큰 획득
	token, err := GetDockerHubToken(auth)
	if err != nil {
		return nil, fmt.Errorf("failed to get Docker Hub token: %v", err)
	}

	// 2. Rate limit 정보 조회
	rateURL := "https://registry-1.docker.io/v2/ratelimitpreview/test/manifests/latest"
	req, err := http.NewRequest("HEAD", rateURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create rate limit request: %v", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get rate limit: %v", err)
	}
	defer resp.Body.Close()

	// rate limit 정보를 헤더에서 추출
	rateLimit := &DockerHubRateLimit{}

	// 윈도우 기간 계산 (초 단위)
	var window int
	if limitHeader := resp.Header.Get("Ratelimit-Limit"); limitHeader != "" {
		if parts := strings.Split(limitHeader, ";w="); len(parts) > 1 {
			window, _ = strconv.Atoi(strings.TrimRight(parts[1], "[]"))
			// window 값을 이용하여 reset time 계산
			rateLimit.Reset = time.Now().Add(time.Duration(window) * time.Second)
		}
	}

	// 대소문자 구분 없이 헤더 검색
	for k, v := range resp.Header {
		if len(v) == 0 {
			continue
		}
		switch strings.ToLower(k) {
		case "ratelimit-limit":
			rateLimit.Limit = parseRateLimit(v[0])
		case "ratelimit-remaining":
			rateLimit.Remaining = parseRateLimit(v[0])
		case "docker-ratelimit-source":
			rateLimit.Source = strings.Trim(v[0], "[]")
		}
	}

	if rateLimit.Limit == 0 && rateLimit.Remaining == 0 && rateLimit.Source == "" {
		return nil, fmt.Errorf("no rate limit information found in response headers")
	}

	return rateLimit, nil
}

// PrintDockerHubRateLimit Docker Hub rate limit 정보를 출력
func PrintDockerHubRateLimit(rateLimit *DockerHubRateLimit, auth DockerHubAuth) {
	// 인증 상태에 따른 메시지 준비
	authStatus := "Anonymous"
	if auth.Username != "" && auth.Password != "" {
		authStatus = "Authenticated"
	}

	fmt.Printf("\nDocker Hub Rate Limits (%s):\n", authStatus)
	fmt.Printf("================================\n")
	fmt.Printf("Limit: %d requests\n", rateLimit.Limit)
	fmt.Printf("Remaining: %d requests\n", rateLimit.Remaining)
	if !rateLimit.Reset.IsZero() {
		fmt.Printf("Reset Time: %s\n", rateLimit.Reset.Format("2006-01-02 15:04:05"))
	}
	fmt.Printf("Source: %s\n", rateLimit.Source)
	fmt.Printf("================================\n\n")
}
