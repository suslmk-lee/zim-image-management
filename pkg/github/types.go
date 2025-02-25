package github

import "time"

// RateLimit GitHub Container Registry의 rate limit 정보
type RateLimit struct {
	Limit     int   `json:"limit"`
	Remaining int   `json:"remaining"`
	Reset     int64 `json:"reset"`
	Used      int   `json:"used"`
}

// GetResetTime Reset 시간을 time.Time 형식으로 반환
func (r *RateLimit) GetResetTime() time.Time {
	return time.Unix(r.Reset, 0)
}
