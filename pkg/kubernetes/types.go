package kubernetes

// ImagePullCount 이미지 풀 횟수를 저장하는 구조체
type ImagePullCount struct {
	Name  string
	Count int
}

// ImagePullCounts 이미지 풀 횟수 슬라이스를 정렬하기 위한 타입
type ImagePullCounts []ImagePullCount

func (a ImagePullCounts) Len() int           { return len(a) }
func (a ImagePullCounts) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ImagePullCounts) Less(i, j int) bool { return a[i].Count > a[j].Count }
