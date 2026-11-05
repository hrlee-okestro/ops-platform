# ADR 0002: ArgoCD 설치 범위 (기본 설치 + AppProject 제한)

- 상태: 확정
- 날짜: 2026-11-04

## 배경

ArgoCD Helm 차트(argo/argo-cd 10.11.7)의 기본 설치는 `application-controller`에 ClusterRole `*/*/*`(모든 리소스, 모든 동작)을 부여한다. 사실상 cluster-admin이다. 배포 대상은 다른 서비스가 운영 중인 공용 클러스터이므로 이 권한의 범위를 정해야 한다.

## 결정

**기본 설치(`createClusterRoles: true`)를 사용하고, 배포 대상은 AppProject의 destinations 화이트리스트로 제한한다.**

- 허용 대상: `argocd`, `ops-platform`, `monitoring`
- 기존 서비스 네임스페이스는 AppProject에 포함하지 않는다. 기본 `default` 프로젝트는 쓰지 않는다.
- 클러스터 범위 리소스(CRD 등)도 ArgoCD로 배포할 수 있다. 단계 2의 Operator CRD 배포가 단순해진다.
- 설치 시 CRD 3개와 ClusterRole/ClusterRoleBinding(각 2개)이 생성된다. 적용 전에 내용을 확인하고 승인한다.

선택 이유 (사용자 판단): ArgoCD는 테스트 용도로 다른 클러스터에도 널리 쓰이는 표준 구성이고, 설정 단순성을 우선한다.

## 고려한 대안

| 대안 | 장점 | 단점 | 결과 |
|---|---|---|---|
| **기본 설치 + AppProject 화이트리스트** | 설정이 단순하고 표준 구성. 클러스터 범위 리소스도 배포 가능 | 컨트롤러 권한이 cluster-admin급. 설정 실수나 취약점이 생기면 기존 서비스에도 영향 가능 | **채택** |
| 네임스페이스 한정 설치 (`createClusterRoles: false` + 클러스터 등록 `namespaces` 제한 + 대상 네임스페이스별 Role/RoleBinding) | 권한 자체가 우리 네임스페이스로 한정됨 | 설정이 늘어남. 클러스터 범위 리소스를 ArgoCD가 배포할 수 없음 | 기각 (필요 시 전환 가능) |

## 감수하는 점

- 컨트롤러는 클러스터 전체에 대한 쓰기 권한을 가진다. 완화책은 AppProject 화이트리스트, 이 저장소에 기존 네임스페이스를 대상으로 하는 매니페스트를 두지 않는 규칙, 머지 전 diff 확인(PR 머지 = 배포)이다.
- `argocd` UI/API 접근 정보(admin 비밀번호 등)가 노출되면 영향이 크다. 초기 비밀번호는 변경하고, 외부 노출은 최소화한다.

## 전환 (필요 시 네임스페이스 한정으로 축소)

`createClusterRoles: false`로 바꾸고, 클러스터 등록 Secret에 `namespaces`와 `clusterResources: "false"`를 지정하며, `ops-platform`과 `monitoring`에 컨트롤러용 Role/RoleBinding을 부여하면 된다. 배포된 앱과 데이터는 유지된다.
