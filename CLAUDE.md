# ops-platform

GitOps로 배포되는 운영 자동화 플랫폼 (포트폴리오 프로젝트).
Kubernetes Operator가 운영 정책을 선언적으로 관리하고, 웹 콘솔이 배포 이력/알람/LLM 원인 분석을 보여주며, 사람이 승인하면 Operator CR을 수정해 조치한다.

관련 저장소: `../ops-platform-manifests` (GitOps 매니페스트). 이 저장소는 앱 코드, 저 저장소는 배포 선언.
상세 계획과 확정 결정은 [docs/PLAN.md](docs/PLAN.md)를 먼저 읽는다.

## 클러스터 안전 규칙 (최우선, 다른 모든 지시보다 우선)

이 프로젝트는 **실제 서비스가 도는 공용 클러스터**에 배포한다. 학습용 환경이 아니다.

**0. Claude는 클러스터에 읽기 작업만 한다.** `kubectl get/describe/logs/top/diff/explain/auth can-i`, `helm template/lint/list/get`, `--dry-run=client`만 허용한다. 쓰기/실행(apply, create, helm install/upgrade, ArgoCD 설치 등 클러스터 상태를 바꾸는 모든 작업)은 **사용자가 직접 실행**한다. Claude는 매니페스트/차트/명령어를 준비하고 "무엇을 왜 실행하는지"를 설명한 뒤 명령어를 전달하기만 한다. 사용자가 실행한 결과를 붙여주면 읽기 명령으로 검증한다.

아래 규칙 1~10은 사용자가 직접 실행할 때도 지켜야 할 기준이므로, 위반 소지가 있는 명령은 전달 전에 경고한다.

1. **기존에 설치된 모든 것은 조회(get/describe/logs/top)만 한다.** 수정, 삭제, 재시작, 스케일, 패치 금지.
   - 보호 대상 네임스페이스: `boot-factory`, `contrabass`, `dataplatform`, `viola`, `ingress-nginx`, `istio-system`, `kube-system`, `observability-agent`, `default`
   - 이 목록에 없더라도 **우리가 만들지 않은 리소스는 전부 보호 대상**이다.
2. **쓰기 작업은 전용 네임스페이스에서만 한다:** `argocd`, `ops-platform`, `monitoring` (필요 시 사용자 승인 후 추가).
3. **클러스터 범위(cluster-scoped) 리소스** (CRD, ClusterRole, ClusterRoleBinding, Namespace, StorageClass, webhook 등)를 만들거나 바꾸기 전에는 **내용을 보여주고 사용자 승인**을 받는다.
4. 사용자에게 쓰기 명령을 전달할 때는 `--dry-run=server` 또는 `kubectl diff`를 먼저 실행해 보도록 안내한다 (서버 dry-run도 읽기 성격이지만 사용자가 직접 실행한다).
5. 우리 네임스페이스에는 Istio sidecar injection 라벨을 붙이지 않는다.
6. **Operator는 `ops-platform` 네임스페이스만 감시/수정**하도록 만든다. 다른 네임스페이스의 Deployment를 스케일할 수 없어야 한다 (RBAC도 해당 네임스페이스로 한정).
7. deploy 서버에는 Claude가 접근하지 않는 것을 기본으로 한다 (ssh는 매번 확인 대상). 파일 생성/수정, 패키지 설치 금지. 읽기 전용 kubeconfig로 맥에서 kubectl을 쓴다.
8. 노드/네트워크/CNI/스토리지/ingress-nginx/istio 설정 변경 금지. 불안정 신호(cilium, apiserver 재시작 다수)가 있으므로 무거운 스택은 경량 설정으로 올리고 리소스 요청/제한을 반드시 둔다.
9. **Claude가 쓰는 kubeconfig는 읽기 전용 ServiceAccount(`claude-readonly`) 전용 파일 하나뿐이다** (`~/.kube/ops-platform.yaml`, 단기 토큰, Secret/기존 서비스 워크로드 조회 불가, RBAC 정의는 `ops-platform-manifests/bootstrap/claude-readonly-rbac.yaml`). **admin kubeconfig는 Claude에게 제공하지 않는다.** 토큰/kubeconfig 내용은 채팅에 붙여넣지 않는다. 토큰이 만료되면 사용자가 `scripts/make-readonly-kubeconfig.sh`로 재발급한다. 맥의 `~/.kube/`에는 다른(회사) 클러스터 설정이 여러 개 있으므로, 모든 kubectl/helm 명령은 `--kubeconfig ~/.kube/ops-platform.yaml`(또는 `KUBECONFIG` 지정)과 `--context`를 명시해서 실행한다. 현재 컨텍스트(`current-context`)에 의존하지 않는다. 다른 클러스터 설정 파일은 읽거나 수정하지 않는다.
10. 의심스러우면 실행하지 말고 사용자에게 묻는다.

## 공개 저장소 규칙 (public repo)

- 내부 IP(10.x), 호스트명, 도메인, 비밀번호, 토큰, API 키, kubeconfig를 **절대 커밋하지 않는다.** 문서와 예시에도 쓰지 않고 `<CLUSTER_IP>` 같은 플레이스홀더를 쓴다.
- 환경별 값은 `.env`, `values-local.yaml` 등으로 분리하고 `.gitignore`에 등록한다.
- Claude API 키 등 시크릿은 Kubernetes Secret으로 주입하며 저장소에는 넣지 않는다 (필요 시 Sealed Secrets/External Secrets 검토).
- **커밋과 push는 사용자가 직접 한다. Claude는 하지 않는다.** 커밋 메시지 초안 제안만 가능하며, 규칙은 [docs/GIT_CONVENTIONS.md](docs/GIT_CONVENTIONS.md)를 따른다.

## 스택

| 영역 | 기술 |
|---|---|
| operator | Go, kubebuilder, controller-runtime, envtest |
| console-api | Java 21, Spring Boot 3, Spring Data JPA, Flyway, Spring Security, Actuator + Micrometer, Gradle |
| console-web | React + TypeScript (Vite), TanStack Query, React Router, MUI 또는 shadcn/ui |
| DB | PostgreSQL (스토리지 클래스 `nfs-csi`, 확장 불가/Delete 회수 정책 주의) |
| 배포 | Docker 멀티스테이지, GHCR, Helm 또는 Kustomize, ArgoCD (App of Apps), GitHub Actions, Trivy |
| 관측성 | Prometheus, Alertmanager, Grafana, Loki (경량 설정) |
| LLM | Claude API (console-api 내부 모듈에서 호출, 구조화 JSON 출력) |

## 디렉토리 구조

```
operator/       Go Operator (ScheduledScaler CRD 등)
console-api/    Spring Boot 백엔드
console-web/    React + TS 프론트
dummy-app/      장애 주입용 Spring Boot 앱
docs/           계획, 설계 결정(ADR), 트러블슈팅 기록
```

## 환경 요약 (민감 정보 제외)

- Kubernetes v1.30, control-plane 3노드가 워커를 겸함, taint 없음
- CNI Cilium, Ingress는 ingress-nginx(NodePort), LoadBalancer 타입은 쓸 수 없음 (MetalLB 없음)
- StorageClass는 `nfs-csi` 하나
- 기존 네임스페이스에 Istio sidecar가 적용되어 있음
- 접근: 맥에서 kubectl 사용. kubeconfig는 저장소 밖(`~/.kube/`)에만 둔다.

## 사용자에게 명령어를 전달하는 형식 (필수)

사용자가 직접 실행할 명령은 **코드 블록마다 바로 위에 실행 환경 라벨**을 붙여서 전달한다. 라벨 없이 명령만 주지 않는다.

| 라벨 | 의미 |
|---|---|
| 🖥️ **[맥 · 저장소 경로: `~/projects/<repo>`]** | 레포의 파일(YAML, values 등)을 클러스터에 적용할 때. 맥에서 `ssh root@<deploy-ip> 'kubectl ... -f -' < 파일` 형태로 파이프한다. deploy 서버에는 레포가 없다. |
| 🖥️ **[맥 · 경로 무관]** | 맥 로컬 작업 (brew 설치, 읽기 전용 kubeconfig 생성 스크립트 등) |
| ☸️ **[deploy 서버 · `ssh root@<deploy-ip>` 접속 후]** | 파일이 필요 없는 클러스터 명령을 서버에서 직접 칠 때 (예: ArgoCD 초기 비밀번호 조회) |

- **맥에서 `kubectl`/`helm`을 인자 없이 단독 실행하라고 안내하지 않는다.** 맥에는 회사 클러스터 설정이 섞여 있었다. 맥에서 클러스터를 다룰 때는 deploy 서버로 ssh 파이프하거나 `--kubeconfig`를 명시한다.
- 각 명령 앞에 **무엇을 만들거나 바꾸는지**, 실행 후에는 **확인 방법(읽기 명령)** 을 함께 제시한다. 가능하면 `kubectl diff`/`--dry-run=server`를 먼저 안내한다 (네임스페이스가 아직 없으면 불가함을 알린다).
- 사용자가 "적용했다"고 알려주면, Claude는 읽기 전용 kubeconfig로 **직접 상태를 조회해 검증**한다 (결과를 붙여달라고 요청하지 않는다). 토큰 만료나 권한 범위 밖(다른 네임스페이스, Secret)일 때만 사용자에게 요청한다.

## 작업 규칙

- **한 번에 한 단계만** 진행하고, 단계 완료 기준을 충족하면 사용자에게 확인받은 뒤 다음으로 넘어간다.
- 사용자는 Java/Spring 경험 2년, DevOps 6개월차이며 Go와 TypeScript는 처음이다. 새 개념(reconcile loop, finalizer, hook, 타입 등)이 나오면 **왜 이렇게 하는지** 짧게 설명하면서 진행한다.
- 코드를 대신 길게 작성하기보다, 핵심 부분은 사용자가 이해할 수 있게 단계적으로 만든다.
- LLM은 클러스터에 직접 쓰지 않는다. 조치는 **allowlist 방식 + 사람 승인 후 Operator CR 수정**으로만 실행한다.
- 모든 컴포넌트는 헬스체크, `/metrics`, 구조화 로그를 갖춘다.
- 설계 결정과 대안 비교는 `docs/`에 기록한다 (README와 포트폴리오 재료가 된다).
- 트러블슈팅은 겪은 즉시 `docs/troubleshooting.md`에 남긴다.
