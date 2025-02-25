package kubernetes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/tabwriter"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetPodImages 클러스터의 모든 Pod에서 사용 중인 이미지 목록을 조회
func GetPodImages(clientset *kubernetes.Clientset) ([]string, error) {
	pods, err := clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %v", err)
	}

	var images []string
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.Containers {
			images = append(images, container.Image)
		}
		// Init 컨테이너의 이미지도 포함
		for _, container := range pod.Spec.InitContainers {
			images = append(images, container.Image)
		}
	}

	return images, nil
}

// cleanImageName 이미지 이름에서 태그와 다이제스트를 제거
func cleanImageName(imageName string) string {
	// 태그 또는 다이제스트 제거
	if strings.Contains(imageName, "@sha256:") {
		parts := strings.Split(imageName, "@sha256:")
		return parts[0]
	}
	if strings.Contains(imageName, ":") {
		parts := strings.Split(imageName, ":")
		return parts[0]
	}
	return imageName
}

// extractImageName 로그 라인에서 이미지 이름 추출
func extractImageName(line string) string {
	parts := strings.Split(line, "Pulled image")
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

// GetPullEvents 지정된 시간 이후의 풀 이벤트를 조회
func GetPullEvents(since string) ([]string, error) {
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

// PrintImagePullStatistics 이미지 풀 통계 출력
func PrintImagePullStatistics(clientset *kubernetes.Clientset, pullEvents []string, since int) error {
	// 현재 클러스터에서 사용 중인 이미지 목록 조회
	clusterImages, err := GetPodImages(clientset)
	//fmt.Println("<><>", clusterImages)
	if err != nil {
		return fmt.Errorf("failed to get cluster images: %v", err)
	}

	// 이미지별 풀 횟수 계산
	imageCounts := make(map[string]int)
	for _, event := range pullEvents {
		imageName := extractImageName(event)
		if imageName != "" {
			imageCounts[imageName]++
		}
	}

	// 정렬을 위해 슬라이스로 변환
	var imagePullCounts []struct {
		Name  string
		Count int
	}
	for name, count := range imageCounts {
		imagePullCounts = append(imagePullCounts, struct {
			Name  string
			Count int
		}{Name: name, Count: count})
	}
	sort.Slice(imagePullCounts, func(i, j int) bool {
		return imagePullCounts[i].Count > imagePullCounts[j].Count
	})

	// 표 형식으로 출력
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.TabIndent)
	fmt.Fprintf(w, "\nImage Pull Statistics (Last %d hours):\n", since)
	fmt.Fprintln(w, "=======================================================================")
	fmt.Fprintln(w, "No.\tImage Name\tPull Count\tIn Use")
	fmt.Fprintln(w, "-----------------------------------------------------------------------")

	for i, img := range imagePullCounts {
		inUse := "No"
		cleanName := cleanImageName(img.Name)
		for _, clusterImg := range clusterImages {
			if cleanImageName(clusterImg) == cleanName {
				inUse = "Yes"
				break
			}
		}
		fmt.Fprintf(w, "%d\t%s\t%d\t%s\n", i+1, img.Name, img.Count, inUse)
	}
	w.Flush()

	// 요약 정보 출력
	var totalPulls int
	var activeImages int
	for _, img := range imagePullCounts {
		totalPulls += img.Count
		cleanName := cleanImageName(img.Name)
		for _, clusterImg := range clusterImages {
			if cleanImageName(clusterImg) == cleanName {
				activeImages++
				break
			}
		}
	}
	fmt.Printf("\nSummary:\n")
	fmt.Printf("- Period: Last %d hours\n", since)
	fmt.Printf("- Total pull events: %d\n", totalPulls)
	fmt.Printf("- Unique images with pulls: %d\n", len(imagePullCounts))
	fmt.Printf("- Currently active images: %d\n", activeImages)

	return nil
}
