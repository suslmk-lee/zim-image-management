package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

// 이미지 풀 카운트를 저장하는 구조체
type ImagePullCount struct {
	Name  string
	Count int
}

// RateLimit GitHub Docker 레지스트리의 rate limit 응답 구조체
type RateLimit struct {
	Limit     int   `json:"limit"`
	Remaining int   `json:"remaining"`
	Reset     int64 `json:"reset"` // Unix timestamp
	Used      int   `json:"used"`
}

// DockerHubRateLimit Docker Hub의 rate limit 정보를 저장하는 구조체
type DockerHubRateLimit struct {
	Limit     int
	Remaining int
	Source    string
	Reset     time.Time
}

// DockerHubAuth Docker Hub 인증 정보
type DockerHubAuth struct {
	Username string
	Password string
	Token    string
}

// GetResetTime Unix timestamp를 time.Time으로 변환
func (r *RateLimit) GetResetTime() time.Time {
	return time.Unix(r.Reset, 0)
}

func main() {
	// 플래그 설정
	kubeconfig := flag.String("kubeconfig", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "absolute path to the kubeconfig file")
	since := flag.String("since", "2024-01-01", "start date for log retrieval (e.g., '2024-01-01')")
	githubToken := flag.String("github-token", "", "GitHub token for checking Docker rate limits")
	checkDockerHub := flag.Bool("check-dockerhub", false, "Check Docker Hub rate limits")
	dockerUsername := flag.String("docker-username", "", "Docker Hub username")
	dockerPassword := flag.String("docker-password", "", "Docker Hub password")
	dockerToken := flag.String("docker-token", "", "Docker Hub token (alternative to username/password)")
	flag.Parse()

	// GitHub Docker rate limit 조회
	if *githubToken != "" {
		rateLimit, err := getDockerRateLimit(*githubToken)
		if err != nil {
			log.Printf("Warning: Failed to get Docker rate limit: %v\n", err)
		} else {
			fmt.Printf("\nGitHub Docker Rate Limits:\n")
			fmt.Printf("================================\n")
			fmt.Printf("Limit: %d\n", rateLimit.Limit)
			fmt.Printf("Remaining: %d\n", rateLimit.Remaining)
			fmt.Printf("Used: %d\n", rateLimit.Used)
			fmt.Printf("Reset Time: %s\n", rateLimit.GetResetTime().Local().Format("2006-01-02 15:04:05"))
			fmt.Printf("================================\n\n")
		}
	}

	// Docker Hub rate limit 조회
	if *checkDockerHub {
		auth := DockerHubAuth{
			Username: *dockerUsername,
			Password: *dockerPassword,
			Token:    *dockerToken,
		}

		_, err := getDockerHubRateLimit(auth)
		if err != nil {
			log.Printf("Warning: Failed to get Docker Hub rate limit: %v\n", err)
		}
	}

	// Kubernetes 클라이언트 설정
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %v", err)
	}

	// 현재 배포된 파드의 이미지 목록 가져오기
	imagesInUse, err := getImagesFromPods(clientset)
	// 이미지 목록을 슬라이스로 변환하여 정렬
	var sortedImages []string
	for image := range imagesInUse {
		sortedImages = append(sortedImages, image)
	}
	sort.Strings(sortedImages)

	if err != nil {
		log.Fatalf("Error retrieving pod images: %v", err)
	}

	// CRI-O 풀 이벤트 로그 가져오기
	pullEvents, err := getPullEvents(*since)
	//fmt.Println("CRI-O pull Event :: ", pullEvents)
	if err != nil {
		log.Fatalf("Error retrieving pull events: %v", err)
	}

	// 이미지별 풀 횟수 맵
	imagePullCounts := make(map[string]int)

	// 풀 이벤트 분석 (배포된 이미지에만 한정)
	for _, event := range pullEvents {
		imageName := extractImageName(event)
		if imageName != "" && imagesInUse[imageName] {
			imagePullCounts[imageName]++
		}
	}

	// 결과를 정렬하기 위한 슬라이스 생성
	var sortedCounts []ImagePullCount
	for image, count := range imagePullCounts {
		sortedCounts = append(sortedCounts, ImagePullCount{Name: image, Count: count})
	}

	// 카운트 내림차순으로 정렬
	sort.Slice(sortedCounts, func(i, j int) bool {
		return sortedCounts[i].Count > sortedCounts[j].Count
	})

	// 표 형식으로 출력
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.TabIndent)
	fmt.Fprintf(w, "\nImage Pull Statistics (since %s):\n", *since)
	fmt.Fprintln(w, "=======================================================================")
	fmt.Fprintln(w, "No.\tImage Name\tPull Count")
	fmt.Fprintln(w, "-----------------------------------------------------------------------")
	for i, img := range sortedCounts {
		fmt.Fprintf(w, "%d\t%s\t%d\n", i+1, img.Name, img.Count)
	}
	fmt.Fprintln(w, "=======================================================================")
	w.Flush()

	var totalPulls int
	for _, img := range sortedCounts {
		totalPulls += img.Count
	}
	fmt.Printf("\nSummary:\n")
	fmt.Printf("- Period: Since %s\n", *since)
	fmt.Printf("- Total pull events: %d\n", totalPulls)
	fmt.Printf("- Unique images with pulls: %d\n", len(imagePullCounts))
}

// 이미지 이름에서 태그와 해시를 제거하고 기본 이미지 이름만 반환
func cleanImageName(image string) string {
	// @sha256 해시 제거
	if shaIndex := strings.Index(image, "@sha256"); shaIndex != -1 {
		image = image[:shaIndex]
	}

	// :tag 제거
	if tagIndex := strings.LastIndex(image, ":"); tagIndex != -1 {
		image = image[:tagIndex]
	}

	return strings.TrimSpace(image)
}

// 현재 배포된 파드에서 사용 중인 이미지 목록 가져오기
func getImagesFromPods(clientset *kubernetes.Clientset) (map[string]bool, error) {
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing pods: %v", err)
	}

	images := make(map[string]bool)
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			cleanedImage := cleanImageName(container.Image)
			if cleanedImage != "" {
				images[cleanedImage] = true
			}
		}
		for _, initContainer := range pod.Spec.InitContainers {
			cleanedImage := cleanImageName(initContainer.Image)
			if cleanedImage != "" {
				images[cleanedImage] = true
			}
		}
	}
	return images, nil
}

// journalctl을 통해 CRI-O 풀 이벤트 로그 가져오기
func getPullEvents(since string) ([]string, error) {
	cmd := exec.Command("journalctl", "-u", "crio", "--since", since, "-g", "pulled image")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("journalctl failed: %v, stderr: %s", err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("error running journalctl: %v", err)
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil, fmt.Errorf("no pull events found in logs since %s", since)
	}
	return lines, nil
}

// 이벤트 로그에서 이미지 이름 추출
func extractImageName(event string) string {
	parts := strings.Split(event, "Pulled image")
	if len(parts) < 2 {
		return ""
	}
	imagePart := strings.TrimSpace(parts[1])

	// 콜론(:) 이후의 부분 추출
	if colonIndex := strings.Index(imagePart, ": "); colonIndex != -1 {
		imagePart = strings.TrimSpace(imagePart[colonIndex+2:])
	}

	// 따옴표 이후의 메타데이터 제거
	if quotesIndex := strings.Index(imagePart, "\""); quotesIndex != -1 {
		imagePart = imagePart[:quotesIndex]
	}

	return cleanImageName(imagePart)
}

// getDockerHubToken Docker Hub 인증 토큰 획득
func getDockerHubToken(auth DockerHubAuth) (string, error) {
	if auth.Token != "" {
		return auth.Token, nil
	}

	// Basic 인증 정보로 토큰 획득
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://auth.docker.io/token?service=registry.docker.io&scope=repository:ratelimitpreview/test:pull", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %v", err)
	}

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

// getDockerHubRateLimit Docker Hub의 rate limit 정보를 조회
func getDockerHubRateLimit(auth DockerHubAuth) (*DockerHubRateLimit, error) {
	client := &http.Client{}

	// 1. 토큰 획득
	authURL := "https://auth.docker.io/token?service=registry.docker.io&scope=repository:ratelimitpreview/test:pull"
	req, err := http.NewRequest("GET", authURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth request: %v", err)
	}

	req.SetBasicAuth(auth.Username, auth.Password)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth token: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("auth request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %v", err)
	}

	// 2. Rate limit 정보 조회
	rateURL := "https://registry-1.docker.io/v2/ratelimitpreview/test/manifests/latest"
	req, err = http.NewRequest("HEAD", rateURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create rate limit request: %v", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", tokenResp.Token))
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	resp, err = client.Do(req)
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

	fmt.Printf("\nDocker Hub Rate Limits:\n")
	fmt.Printf("================================\n")
	fmt.Printf("Limit: %d requests\n", rateLimit.Limit)
	fmt.Printf("Remaining: %d requests\n", rateLimit.Remaining)
	if window > 0 {
		fmt.Printf("Window: %d hours\n", window/3600)
		fmt.Printf("Reset Time: %s\n", rateLimit.Reset.Format("2006-01-02 15:04:05"))
	}
	fmt.Printf("Source: %s\n", rateLimit.Source)
	fmt.Printf("================================\n\n")

	return rateLimit, nil
}

// DockerRateLimit GitHub Docker 레지스트리의 rate limit 정보를 조회
func getDockerRateLimit(token string) (*RateLimit, error) {
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var rateLimit struct {
		Resources struct {
			Core RateLimit `json:"core"`
		} `json:"resources"`
	}

	if err := json.Unmarshal(body, &rateLimit); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v\nResponse body: %s", err, string(body))
	}

	return &rateLimit.Resources.Core, nil
}
