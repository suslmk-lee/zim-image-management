package kubernetes

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
)

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
	re := regexp.MustCompile(`"image":"([^"]+)"`)
	matches := re.FindStringSubmatch(line)
	if len(matches) < 2 {
		return ""
	}
	imagePart := matches[1]
	return cleanImageName(imagePart)
}

// GetPullEvents 지정된 날짜 이후의 풀 이벤트를 조회
func GetPullEvents(since string) ([]string, error) {
	cmd := exec.Command("journalctl", "-u", "crio", "--since", since, "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute journalctl: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	var pullEvents []string
	for _, line := range lines {
		if strings.Contains(line, "Pulling image") {
			pullEvents = append(pullEvents, line)
		}
	}

	return pullEvents, nil
}

// PrintImagePullStatistics 이미지 풀 통계 출력
func PrintImagePullStatistics(pullEvents []string, since int) {
	// 이미지별 풀 횟수 계산
	imageCounts := make(map[string]int)
	for _, event := range pullEvents {
		imageName := extractImageName(event)
		if imageName != "" {
			imageCounts[imageName]++
		}
	}

	// 정렬을 위해 슬라이스로 변환
	var imagePullCounts ImagePullCounts
	for name, count := range imageCounts {
		imagePullCounts = append(imagePullCounts, ImagePullCount{Name: name, Count: count})
	}
	sort.Sort(imagePullCounts)

	// 표 형식으로 출력
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.TabIndent)
	fmt.Fprintf(w, "\nImage Pull Statistics (Last %d hours):\n", since)
	fmt.Fprintln(w, "=======================================================================")
	fmt.Fprintln(w, "No.\tImage Name\tPull Count")
	fmt.Fprintln(w, "-----------------------------------------------------------------------")
	
	for i, img := range imagePullCounts {
		fmt.Fprintf(w, "%d\t%s\t%d\n", i+1, img.Name, img.Count)
	}
	w.Flush()

	// 요약 정보 출력
	var totalPulls int
	for _, img := range imagePullCounts {
		totalPulls += img.Count
	}
	fmt.Printf("\nSummary:\n")
	fmt.Printf("- Period: Last %d hours\n", since)
	fmt.Printf("- Total pull events: %d\n", totalPulls)
	fmt.Printf("- Unique images with pulls: %d\n", len(imagePullCounts))
}
