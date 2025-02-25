package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/suslmk-lee/zim-image-management/pkg/docker"
	"github.com/suslmk-lee/zim-image-management/pkg/github"
	"github.com/suslmk-lee/zim-image-management/pkg/kubernetes"
)

func printUsage() {
	fmt.Printf(`ZIM (Zim Image Management) - Docker Image Usage Monitor

Usage: %s [options]

Options:
  --kubeconfig string
        Path to kubeconfig file (default: $HOME/.kube/config)
  --since int
        Show statistics for the last N hours (default: 24)
  --github-token string
        GitHub personal access token for checking GitHub Container Registry rate limits
  --docker-username string
        Docker Hub username for authenticated rate limit checking
  --docker-password string
        Docker Hub password for authenticated rate limit checking
  --docker-token string
        Docker Hub token (alternative to username/password)
  --version
        Show version information

Examples:
  # Show image pull statistics for the last 24 hours
  %s

  # Show image pull statistics for the last 48 hours
  %s --since 48

  # Check Docker Hub rate limits with authentication
  %s --docker-username user --docker-password pass

  # Check GitHub Container Registry rate limits
  %s --github-token ghp_xxxxxxxxxxxx
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}

func main() {
	// 커스텀 usage 메시지 설정
	flag.Usage = printUsage

	// 플래그 설정
	kubeconfig := flag.String("kubeconfig", filepath.Join(os.Getenv("HOME"), ".kube", "config"),
		"Absolute path to the kubeconfig file")
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

	// Kubernetes 클라이언트 생성
	kubeClient, err := kubernetes.NewKubeClient(*kubeconfig)
	if err != nil {
		log.Fatalf("Failed to create kubernetes client: %v", err)
	}

	// GitHub Container Registry rate limit 확인
	if *githubToken != "" {
		githubLimit, err := github.GetDockerRateLimit(*githubToken)
		if err != nil {
			log.Printf("Warning: Failed to get GitHub Container Registry rate limit: %v", err)
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
	}

	if err != nil {
		log.Printf("Warning: Failed to get Docker Hub rate limit: %v", err)
	} else {
		docker.PrintDockerHubRateLimit(dockerLimit, auth)
	}

	// 시간 범위 계산
	sinceTime := fmt.Sprintf("%dh ago", *since)

	// 이미지 풀 이벤트 조회
	pullEvents, err := kubernetes.GetPullEvents(sinceTime)
	//pullEvents := []string{
	//	`{"_GID":"0","MESSAGE":"time=\"2025-02-24 08:58:41.057826731+09:00\" level=info msg=\"Pulling image: quay.io/calico/cni:v3.28.1\" id=e9ccf295-4f1a-44c3-bd87-072e86392509 name=/runtime.v1.ImageService/PullImage","_CMDLINE":"/usr/local/bin/crio","SYSLOG_FACILITY":"3","_SYSTEMD_INVOCATION_ID":"df1c3605c5c1422b80a8a8caa88bb5e4","_MACHINE_ID":"b7c627b40f884efc89cc558b646bb208","_UID":"0","__REALTIME_TIMESTAMP":"1740355121057861","SYSLOG_IDENTIFIER":"crio","_EXE":"/usr/local/bin/crio","_SYSTEMD_UNIT":"crio.service","_SYSTEMD_CGROUP":"/system.slice/crio.service","_STREAM_ID":"25e6d598787e4500a8b8b4db3307ab78","_BOOT_ID":"ee0f5c9b45ae4939b8411b81845a4d77","_HOSTNAME":"cp-dev-cluster1","_CAP_EFFECTIVE":"1ffffffffff","PRIORITY":"6","_SELINUX_CONTEXT":"unconfined\n","__MONOTONIC_TIMESTAMP":"28458785","__CURSOR":"s=b38820a2e8d34be3b55b8e061bb9e57f;i=a7f64;b=ee0f5c9b45ae4939b8411b81845a4d77;m=1b23f21;t=62ed800c61045;x=330c693a2769fdfd","_SYSTEMD_SLICE":"system.slice","_COMM":"crio","_TRANSPORT":"stdout","_PID":"581"}`,
	//}
	fmt.Println("Pull Events:", pullEvents)
	if err != nil {
		log.Fatalf("Failed to get pull events: %v", err)
	}

	// 이미지 풀 통계 출력
	if err := kubernetes.PrintImagePullStatistics(kubeClient.GetClientset(), pullEvents, *since); err != nil {
		log.Fatalf("Failed to print image pull statistics: %v", err)
	}
}
