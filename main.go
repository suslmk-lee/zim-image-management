package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// 플래그 설정
	kubeconfig := flag.String("kubeconfig", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "absolute path to the kubeconfig file")
	since := flag.String("since", "2024-01-01", "start date for log retrieval (e.g., '2024-01-01')")
	flag.Parse()

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
	if err != nil {
		log.Fatalf("Error retrieving pod images: %v", err)
	}

	// CRI-O 풀 이벤트 로그 가져오기
	pullEvents, err := getPullEvents(*since)
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

	// 결과 출력
	fmt.Println("Image Pull Counts for Deployed Pods (Estimated from CRI-O Events):")
	fmt.Println("----------------------------------------------------------------")
	totalPulls := 0
	for image, count := range imagePullCounts {
		fmt.Printf("%s: %d\n", image, count)
		totalPulls += count
	}
	fmt.Println("----------------------------------------------------------------")
	fmt.Printf("Total pull events for deployed images: %d\n", totalPulls)
	fmt.Printf("Unique deployed images with pulls: %d\n", len(imagePullCounts))
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
			images[container.Image] = true
		}
		for _, initContainer := range pod.Spec.InitContainers {
			images[initContainer.Image] = true
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
	parts := strings.Split(event, "pulled image")
	if len(parts) < 2 {
		return ""
	}
	imagePart := strings.TrimSpace(parts[1])
	end := strings.Index(imagePart, " at ")
	if end == -1 {
		end = len(imagePart)
	}
	imageName := strings.TrimSpace(imagePart[:end])
	if imageName == "" {
		return ""
	}
	return imageName
}
