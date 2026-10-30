# 프로젝트 계획

## 한 줄 요약

GitOps로 배포되는 운영 콘솔에서 알람 원인을 LLM이 분석하고, 사람이 승인하면 직접 만든 Kubernetes Operator의 CR을 수정해 조치까지 이어지는 자동화 플랫폼.

## 목표

- DevOps 포트폴리오: GitOps + Operator 개발 + 웹 콘솔 + LLM 운영 자동화를 하나의 스토리로 연결
- 학습: Go(Operator), TypeScript(콘솔 프론트)를 새로 익힘. Java/Spring 경험은 콘솔 백엔드에 활용

## 아키텍처

```
[개발자] -> PR -> GitHub Actions -> 이미지 빌드/스캔 -> 매니페스트 repo 태그 갱신
                                                          |
                                                    ArgoCD가 배포
                                                          v
  Kubernetes (공용 클러스터, 전용 네임스페이스만 사용)
   dummy-app(장애 주입)    Ops Console (web + api)
        |                       ^      |
   Prometheus/Loki --알람--> console-api --> LLM 분석(Claude API)
                                |
                           PostgreSQL
                                | 승인 시
                                v
                     Operator (CR 수정 -> 조치)
```

원칙: LLM은 클러스터에 직접 쓰지 않는다. 허용 목록(allowlist)에 있는 조치만, 사람이 승인한 뒤 Operator CR 수정으로 실행한다.

## 확정된 결정

| 항목 | 결정 | 이유 |
|---|---|---|
| Operator 언어 | Go + kubebuilder | 업계 표준, 새로 학습 |
| 콘솔 백엔드 | Java 21 + Spring Boot 3 | 친숙한 스택 |
| 콘솔 프론트 | React + TypeScript (Vite) | 트렌드, 새로 학습 |
| 배포 대상 | 기존 공용 클러스터의 전용 네임스페이스 | kind 불필요, 기존 워크로드는 건드리지 않음 |
| 저장소 | 앱 모노레포 + 매니페스트 저장소 분리 | GitOps 표준 관행 |
| 공개 여부 | public | 포트폴리오 |
| LLM 호출 위치 | console-api 내부 모듈 | 초기 단순화 |

## 환경 제약 (민감 정보 제외)

- 공용 클러스터이며 기존 서비스가 운영 중. 기존 리소스는 조회만 한다 (CLAUDE.md 참고)
- LoadBalancer 불가(MetalLB 없음). 외부 노출은 ingress-nginx(NodePort) 사용
- StorageClass는 NFS 기반 하나. DB 성능과 데이터 보존에 주의
- 일부 컨트롤플레인 컴포넌트 재시작 이력이 많아 클러스터가 다소 불안정. 관측성 스택은 경량으로
- 노드 메모리 여유가 크지 않으므로 모든 워크로드에 requests/limits 설정

## 단계

| 단계 | 내용 | 예상 소요 (주 10시간) | 완료 기준 |
|---|---|---|---|
| 0 | 준비: 도구 설치, 저장소 골격, Go/TS 기초 | 1주 | 로컬 환경 구동, 모듈 골격 빌드 |
| 1 | GitOps 기반: ArgoCD 설치, dummy-app 배포, CI | 3~4주 | PR 머지만으로 클러스터에 자동 반영 |
| 2 | Operator(ScheduledScaler) 개발 | 6~8주 | CR 적용 시 스케일 동작, 삭제 시 정리, 테스트 통과 |
| 3 | 콘솔 MVP: 배포 이력 + 정책 관리 | 5~6주 | 화면에서 배포 이력 조회, 정책 수정 |
| 4 | 관측성 + 인시던트 | 3~4주 | 장애 주입 -> 알람 -> 콘솔에 인시던트 표시 |
| 5 | LLM 분석 + 승인 | 3~4주 | 알람 -> 분석 -> 승인 -> 실제 조치 |
| 6 | 마무리: README, 다이어그램, 데모, 블로그 | 2주 | 공개 가능한 문서와 영상 |

중간 종료 지점: 2단계 종료(GitOps + Operator), 3단계 종료(웹 콘솔 포함, 지원 가능한 수준), 5단계 종료(풀 스코프). 일정이 밀리면 4~5단계를 줄이고 3단계까지의 품질을 높인다.

## 단계별 상세

### 단계 0. 준비
- Go, Java 21, Node, Docker, kubectl, Helm, kubebuilder 설치 (맥)
- 맥에서 클러스터 접근용 kubeconfig 구성 (저장소 밖 `~/.kube/`에 보관, 커밋 금지)
- 저장소 골격: `operator/`(go mod), `console-api/`(Gradle), `console-web/`(Vite), `dummy-app/`
- 전용 네임스페이스 생성 계획 수립 (`argocd`, `ops-platform`, `monitoring`)
- Go, TypeScript 기초 학습 병행

### 단계 1. GitOps 기반
- ArgoCD를 `argocd` 네임스페이스에 설치 (UI는 ingress-nginx로 노출)
- dummy-app 이미지 빌드 후 GHCR push, 매니페스트 저장소로 배포 (App of Apps)
- GitHub Actions: 테스트 -> 이미지 빌드/푸시 -> Trivy 스캔 -> 매니페스트 이미지 태그 갱신
- 노드에서 GHCR pull이 되는지 확인 (기존 ImagePullBackOff 사례 참고)

### 단계 2. Operator (ScheduledScaler)
- CRD 설계: spec(대상 Deployment, 스케줄, 시간대, 축소 replicas), status(현재 상태, 마지막 동작, conditions)
- Reconciler: 스케줄 평가 -> 스케일 조정 -> 다음 재조정 시각 예약
- finalizer, 이벤트 발행, 멱등성, 수동 스케일 변경과의 충돌 처리
- envtest 단위 테스트, 최소 RBAC, 감시 네임스페이스 제한, 패키징
- ArgoCD로 Operator 자체도 GitOps 배포

### 단계 3. 콘솔 MVP
- DB 스키마(배포 이벤트, 정책 스냅샷), REST API(OpenAPI)
- 배포 이력 수집(GitHub Actions/ArgoCD webhook), 정책 관리 API(CR만 다루도록 ServiceAccount 제한)
- 프론트: 로그인, 배포 이력 타임라인, 정책 목록/편집
- `/actuator/prometheus` 노출, GitOps로 배포

### 단계 4. 관측성 + 인시던트
- Prometheus, Alertmanager, Loki, Grafana를 경량으로 GitOps 설치 (기존 observability-agent와 충돌 없이 분리)
- dummy-app 장애 시나리오(에러율, 지연, OOM)와 알람 룰
- Alertmanager webhook -> console-api 인시던트 저장, 상태 흐름(신규/분석중/조치대기/해결)
- 인시던트 시점의 로그, 이벤트, 최근 배포 이력 수집

### 단계 5. LLM 분석 + 승인
- 수집 컨텍스트를 LLM에 전달, 원인 가설/근거/제안 조치를 JSON으로 수신
- 제안 조치는 allowlist만 (예: ScheduledScaler 수정, 롤백 요청)
- 승인/거절 화면, 승인 시 Operator CR 수정, 분석 결과와 근거 저장, 감사 로그
- 환각 검증을 위해 항상 근거(로그 라인, 이벤트)와 함께 저장

### 단계 6. 마무리
- 아키텍처 다이어그램, README(설계 결정과 대안 비교), 데모 영상, 지표 정리, 트러블슈팅 기록, 블로그 글

## 포트폴리오 산출물 체크리스트

- [ ] 아키텍처 다이어그램
- [ ] README: 문제 정의, 설계 결정과 대안 비교, 실행 방법
- [ ] 데모 영상 (PR 머지 -> 배포 -> 스케일다운 동작 -> 알람 -> 분석 -> 승인)
- [ ] 정량 지표 (배포 시간, 스케일다운으로 절감한 리소스 추정치)
- [ ] 트러블슈팅 기록
- [ ] 블로그 2~3편
