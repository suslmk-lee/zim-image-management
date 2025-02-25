package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// 이미지 풀 카운트를 저장하는 구조체
type ImagePullCount struct {
	Name  string
	Count int
}

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
