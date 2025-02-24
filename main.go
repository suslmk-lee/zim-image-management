package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// kubeconfig 파일 경로 플래그
	kubeconfig := flag.String("kubeconfig", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "absolute path to the kubeconfig file")
	flag.Parse()

	// kubeconfig로 클라이언트 설정
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Fatalf("Error building kubeconfig: %v", err)
	}

	// Kubernetes 클라이언트 생성
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %v", err)
	}

	// 이미지 카운트 맵 초기화
	imageCounts := make(map[string]int)

	// 모든 네임스페이스의 파드 조회
	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Error listing pods: %v", err)
	}

	// 모든 파드의 컨테이너 이미지 수집
	for _, pod := range pods.Items {
		// 기본 컨테이너
		for _, container := range pod.Spec.Containers {
			imageCounts[container.Image]++
		}
		// 초기화 컨테이너
		for _, initContainer := range pod.Spec.InitContainers {
			imageCounts[initContainer.Image]++
		}
	}

	// 결과 출력
	fmt.Println("Kubernetes Image Pull Counts:")
	fmt.Println("-----------------------------")
	for image, count := range imageCounts {
		fmt.Printf("%s: %d\n", image, count)
	}

	// 총 이미지 수와 고유 이미지 수 출력
	totalImages := 0
	for _, count := range imageCounts {
		totalImages += count
	}
	fmt.Println("-----------------------------")
	fmt.Printf("Total images: %d\n", totalImages)
	fmt.Printf("Unique images: %d\n", len(imageCounts))
}

// 사용법 설명을 위한 함수
func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Flags:\n")
	flag.PrintDefaults()
}
