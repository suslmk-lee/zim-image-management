package docker

import "time"

// DockerHubAuth Docker Hub 인증 정보
type DockerHubAuth struct {
	Username string
	Password string
	Token    string
}

// DockerHubRateLimit Docker Hub의 rate limit 정보를 저장하는 구조체
type DockerHubRateLimit struct {
	Limit     int
	Remaining int
	Source    string
	Reset     time.Time
}
