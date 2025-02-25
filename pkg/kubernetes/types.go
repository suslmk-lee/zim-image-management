package kubernetes

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeClient Kubernetes 클라이언트 래퍼
type KubeClient struct {
	clientset *kubernetes.Clientset
}

// NewKubeClient creates a new KubeClient
func NewKubeClient(kubeconfigPath string) (*KubeClient, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &KubeClient{
		clientset: clientset,
	}, nil
}

// GetClientset returns the underlying Kubernetes clientset
func (k *KubeClient) GetClientset() *kubernetes.Clientset {
	return k.clientset
}

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
