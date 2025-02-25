# ZIM (Zim Image Management)

ZIM은 Kubernetes 클러스터의 컨테이너 이미지 사용 현황을 모니터링하고 Docker Hub와 GitHub Container Registry의 rate limit을 확인하는 도구입니다.

## 주요 기능

- 클러스터 내 사용 중인 컨테이너 이미지 목록 조회
- 이미지 풀 이벤트 통계 제공
  - 이미지별 풀 횟수
  - 현재 사용 중인 이미지 표시
- Docker Hub Rate Limit 확인
  - 인증된 사용자와 익명 사용자 지원
  - 토큰 또는 사용자명/비밀번호 인증 지원
- GitHub Container Registry Rate Limit 확인

## 설치 방법

```bash
go install github.com/suslmk-lee/zim-image-management/cmd/zim@latest
```

## 사용 방법

```bash
# 기본 사용법 (최근 24시간 통계)
zim

# 특정 시간 범위 지정
zim --since 48

# Docker Hub 인증 정보 제공
zim --docker-username <username> --docker-password <password>
# 또는
zim --docker-token <token>

# GitHub 토큰 제공
zim --github-token <token>

# 버전 정보 확인
zim --version

# 도움말 보기
zim --help
```

## 요구사항

- Go 1.20 이상
- Kubernetes 클러스터 접근 권한
- CRI-O 컨테이너 런타임 (이미지 풀 이벤트 로그 수집용)
- `journalctl` 명령어 접근 권한

## 출력 예시

```
Image Pull Statistics (Last 24 hours):
=======================================================================
No.     Image Name       Pull Count       In Use
-----------------------------------------------------------------------
1       nginx:1.21       10              Yes
2       redis:6.2        5               Yes
3       mysql:8.0        3               No

Summary:
- Period: Last 24 hours
- Total pull events: 18
- Unique images with pulls: 3
- Currently active images: 2
```

## 라이선스

MIT License