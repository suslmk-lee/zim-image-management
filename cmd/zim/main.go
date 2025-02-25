package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/suslmk-lee/zim-image-management/pkg/docker"
	"github.com/suslmk-lee/zim-image-management/pkg/github"
	"github.com/suslmk-lee/zim-image-management/pkg/kubernetes"
)

func printUsage() {
	fmt.Printf(`ZIM (Zim Image Management) - Docker Image Management Tool

Description:
  ZIM helps you monitor Docker image usage in your Kubernetes cluster and check rate limits
  for both Docker Hub and GitHub Container Registry.

Usage:
  zim-image-management [options]

Options:
`)
	flag.PrintDefaults()
	fmt.Printf(`
Examples:
  1. Show image pull statistics for the last 24 hours (default):
     $ zim-image-management

  2. Show image pull statistics for the last 48 hours:
     $ zim-image-management --since 48

  3. Check GitHub Container Registry rate limits:
     $ zim-image-management --github-token YOUR_GITHUB_TOKEN

  4. Check Docker Hub rate limits with authentication:
     $ zim-image-management --docker-username USER --docker-password PASS

  5. Check Docker Hub rate limits anonymously:
     $ zim-image-management

Note:
  - Docker Hub rate limits will always be checked (authenticated if credentials are provided, 
    otherwise anonymously)
  - All times are displayed in local timezone
  - The kubeconfig file is read from $HOME/.kube/config by default
`)
}

func main() {
	// 커스텀 usage 메시지 설정
	flag.Usage = printUsage

	// 플래그 설정
	since := flag.Int("since", 24, 
		"Show statistics for the last N hours (default: 24)")
	githubToken := flag.String("github-token", "", 
		"GitHub personal access token for checking GitHub Container Registry rate limits")
	dockerUsername := flag.String("docker-username", "", 
		"Docker Hub username for authenticated rate limit checking")
	dockerPassword := flag.String("docker-password", "", 
		"Docker Hub password for authenticated rate limit checking")
	dockerToken := flag.String("docker-token", "", 
		"Docker Hub token (alternative to username/password)")

	// 버전 플래그 추가
	version := flag.Bool("version", false, 
		"Show version information")

	flag.Parse()

	// 버전 정보 출력
	if *version {
		fmt.Println("ZIM (Zim Image Management) version 1.0.0")
		os.Exit(0)
	}

	// GitHub Container Registry rate limit 확인
	if *githubToken != "" {
		githubLimit, err := github.GetDockerRateLimit(*githubToken)
		if err != nil {
			log.Printf("Warning: Failed to get GitHub Docker rate limit: %v\n", err)
		} else {
			github.PrintGitHubRateLimit(githubLimit)
		}
	}

	// Docker Hub rate limit 확인 (인증된 사용자 또는 익명)
	auth := docker.DockerHubAuth{
		Username: *dockerUsername,
		Password: *dockerPassword,
		Token:    *dockerToken,
	}

	dockerLimit, err := docker.GetDockerHubRateLimit(auth)
	if err != nil {
		log.Printf("Warning: Failed to get Docker Hub rate limit: %v\n", err)
	} else {
		docker.PrintDockerHubRateLimit(dockerLimit, auth)
	}

	// 이미지 풀 통계 조회
	sinceTime := time.Now().Add(-time.Duration(*since) * time.Hour).Format("2006-01-02")
	pullEvents, err := kubernetes.GetPullEvents(sinceTime)
	if err != nil {
		log.Fatalf("Error retrieving pull events: %v", err)
	}

	kubernetes.PrintImagePullStatistics(pullEvents, *since)
}
