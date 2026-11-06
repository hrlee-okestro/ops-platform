# 트러블슈팅 기록

겪은 즉시 남긴다. 포트폴리오와 블로그 재료다. 내부 IP와 호스트명은 쓰지 않는다 (`<...>` 플레이스홀더 사용).

---

## 1. ArgoCD가 GitHub에 접속하지 못함: `lookup github.com ... server misbehaving` (2026-11-05)

### 증상
ArgoCD `root` Application이 `Sync: Unknown`이고 조건에 다음 오류가 표시됨.

```
failed to list refs: Get "https://github.com/<owner>/<repo>.git/info/refs?service=git-upload-pack":
dial tcp: lookup github.com on <cluster-dns-ip>:53: server misbehaving
```

### 원인 (확인한 순서)
1. ArgoCD repo-server 파드 안에서 `getent hosts github.com`이 실패했다. 점을 붙인 `github.com.`도 실패해서 검색 도메인(`ndots`) 문제가 아니었다.
2. 같은 파드에서 클러스터 내부 이름(`<svc>.<ns>.svc.cluster.local`)은 풀렸다. 즉 CoreDNS 자체는 정상이고 **외부 이름 전달(forward)** 쪽 문제였다.
3. deploy 서버(클러스터 밖)에서는 `github.com`이 잘 풀렸다. 노드와 파드의 DNS 경로가 다르다.
4. 1회용 파드(`busybox`)를 우리 네임스페이스에 띄워 DNS 서버별로 질의했다.
   - 사내(OpenStack) DNS 3대: 모두 `REFUSED` (파드에서 오는 질의를 거절)
   - `8.8.8.8`: 성공
5. CoreDNS Corefile이 `forward . /etc/resolv.conf`로 외부 이름을 노드의 DNS 설정(사내 DNS)에 넘기고 있어서, 사내 DNS가 거절하면 외부 이름이 전부 실패했다.

결과: 이 클러스터의 **파드는 외부 이름을 전혀 해석하지 못하는 상태**였다. ArgoCD→GitHub뿐 아니라 이후 GHCR, Claude API 호출도 같은 문제를 겪을 구조였다.

### 시도하지 않은 방법과 이유
- 노드의 `/etc/resolv.conf` 직접 수정: systemd-resolved가 관리해서 덮어씌워지고, 노드 3대와 OS 설정은 클러스터 소유 측 영역이다. 노드 자신의 DNS와 이미지 pull에도 영향이 간다.
- 우리 파드에만 `dnsConfig`/`ndots` 지정: 원인(업스트림 거절)을 해결하지 못하고 워크로드마다 반복해야 한다.

### 조치
`kube-system`의 CoreDNS ConfigMap에서 `forward` 대상을 변경 (사용자 허용, 사용자가 직접 실행).

```
forward . /etc/resolv.conf { ... }   →   forward . 8.8.8.8 8.8.4.4 { ... }
```

절차: 현재 ConfigMap 백업 → `sed`로 수정본 생성 → `diff`로 한 줄만 바뀐 것 확인 → `kubectl replace`.
CoreDNS의 `reload` 플러그인이 재시작 없이 설정을 반영했다.

되돌리기: `kubectl replace -f <백업 파일>`

### 검증
- repo-server에서 `getent hosts github.com` 성공, 클러스터 내부 이름도 성공
- CoreDNS 파드 2개 Running, 재시작 없음, 로그에 오류 없음
- 정상이 아닌 파드는 변경 전부터 있던 것 하나뿐 (`viola-api-admin` CrashLoopBackOff, 69일 전부터)
- ArgoCD 오류가 `app path does not exist`(저장소의 `apps/`를 아직 push하지 않음)로 바뀜: GitHub 접속 성공

### 알아둘 점 (리스크)
- 영향 범위가 클러스터 전체의 외부 이름 해석이다. 사내 DNS로만 풀리던 이름이 있다면 영향이 있을 수 있다 (파드에서 사내 DNS가 거절하던 점으로 보아 이미 실패했을 가능성이 높지만 확정할 수 없음).
- 이 Corefile은 클러스터 설치 도구가 관리하는 값이라 소유 측이 재설치/업그레이드하면 원래대로 돌아갈 수 있다.
- 관련 관찰: 노드의 DNS 서버가 3개를 넘어 `DNSConfigForming` 경고가 컨트롤플레인 파드에서 발생한다. 이번 변경과 무관해 보이며 소유 측에 전달할 만한 정보다.
- 더 좁은 대안: 필요한 도메인(`github.com`, `ghcr.io`, `githubusercontent.com`, `github.io`, `anthropic.com`)만 별도 서버 블록으로 8.8.8.8에 보내면 기존 파드 영향을 최소화할 수 있다. 문제가 생기면 이 방식으로 전환을 검토한다.

### 배운 점
- `server misbehaving`은 "응답 없음"이 아니라 DNS 서버가 오류로 답했다는 뜻이다. CoreDNS까지는 도달했는지, 그 뒤 업스트림이 문제인지 구분해야 한다.
- 클러스터 밖 서버에서의 성공은 파드 안에서의 성공을 보장하지 않는다. 파드 네트워크에서 직접 재현하는 1회용 파드가 가장 빠른 진단 수단이었다.
- 가설(검색 도메인)을 먼저 세웠지만 점 붙인 이름 테스트로 반증했다. 값싼 반례 테스트 후에 수정했다.

---

## 2. GitHub Actions `image` 잡 실패: Trivy 액션 해석 실패와 취약점 게이트 (2026-11-06)

### 증상
dummy-app CI의 `test`는 통과하지만 `image` 잡이 실패했다. 같은 잡이 서로 다른 이유로 세 번 실패했다.

| 회차 | 실패 단계 | 메시지 |
|---|---|---|
| 1 | Set up job | `Unable to resolve action aquasecurity/trivy-action@0.28.0, unable to find version` |
| 2 | Set up job | `Unable to resolve action aquasecurity/setup-trivy@v0.2.1, unable to find version` |
| 3 | Trivy scan | `Process completed with exit code 1` |

### 원인
1. 액션 태그는 `v0.28.0`처럼 `v` 접두사가 붙는다. 접두사 없이 적어서 해석에 실패했다.
2. `trivy-action`은 내부에서 `setup-trivy@v0.2.1`을 다시 호출하는데, 그 태그가 사라져 있었다. 우리 파일을 고쳐도 외부 액션의 내부 의존성은 통제할 수 없다.
3. 이 실패는 **오류가 아니라 게이트가 정상 동작한 것**이다. 베이스 이미지(Ubuntu)의 `apt`에 수정 버전이 나와 있는 HIGH 취약점 2건이 있어서 push 전에 중단됐다.

### 진단
- 실행 상태와 실패 단계는 공개 저장소의 Actions API(`/actions/runs/<id>/jobs`)로, 시작 단계 오류 문구는 `/check-runs/<job-id>/annotations`로 확인했다 (로그 본문은 로그인이 필요).
- 3회차는 같은 이미지(이미 push된 `0.0.1`)를 로컬에서 같은 옵션으로 스캔해 취약점 표를 재현했다.

### 조치
1. 액션 대신 **Trivy 공식 이미지를 `docker run`으로 직접 실행**하도록 워크플로를 바꿨다. 외부 액션 의존이 사라진다.
2. Dockerfile 실행 단계에 `apt-get install --only-upgrade apt libapt-pkg7.0`를 추가했다. 베이스가 갱신돼 이미 수정판이면 아무 일도 하지 않는다.
3. push 전에 amd64로 로컬 빌드해 같은 스캔으로 0건을 확인했다.

### 검증
Actions `test`/`image` 성공 → manifests에 `chore(manifests): bump dummy-app to <sha>` 커밋 → ArgoCD Synced/Healthy → 파드가 새 태그로 롤링 교체.

### 배운 점
- 외부 액션은 태그가 아니라 커밋 SHA로 고정하고, 가능하면 의존이 적은 방식(공식 이미지 직접 실행)이 낫다.
- 보안 스캔이 실패하면 "CI 오류"가 아니라 "차단된 이유"부터 읽는다. 재현은 결과를 push하기 전에 로컬에서 한다.
- 맥 Docker Desktop은 소켓 마운트로 Trivy가 이미지를 찾지 못했다. `docker save`로 tar를 만들어 `--input`으로 스캔하면 된다.
