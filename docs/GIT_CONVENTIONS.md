# Git 커밋/브랜치 규칙

## 원칙

- **커밋과 push는 사용자가 직접 한다.** Claude는 `git commit`, `git push`를 실행하지 않는다 (권한 설정으로도 차단).
- Claude는 요청 시 **커밋 메시지 초안**과 **커밋 단위 분할 제안**만 한다.
- 이 저장소들은 **public**이다. 커밋 전에 민감 정보 점검을 반드시 한다 (아래 체크리스트).

## 커밋 메시지: Conventional Commits

```
<type>(<scope>): <subject>

<body: 왜 바꿨는지, 선택적>
```

| type | 용도 |
|---|---|
| feat | 기능 추가 |
| fix | 버그 수정 |
| docs | 문서 |
| refactor | 동작 변경 없는 구조 개선 |
| test | 테스트 추가/수정 |
| ci | CI/CD 파이프라인 |
| build | 빌드, 의존성 |
| chore | 기타 잡무 |
| perf | 성능 개선 |
| revert | 되돌리기 |

**scope**: `operator`, `console-api`, `console-web`, `dummy-app`, `ci`, `docs`, `manifests`, `argocd`, `monitoring`, `deps`

**subject 규칙**
- 영어, 소문자 시작, 명령형, 50자 이내, 마침표 없음 (언어는 필요하면 팀에서 바꿔도 된다)
- 본문은 한국어 가능. "무엇"보다 "왜"를 적는다.

**예시**

```
feat(operator): add ScheduledScaler CRD and reconciler skeleton
fix(operator): requeue after scale to avoid reconcile hot loop
ci: scan image with trivy before pushing to ghcr
docs: add reconcile loop design decision record
chore(manifests): bump dummy-app image to <short-sha>
```

매니페스트 저장소의 이미지 태그 갱신 커밋은 CI가 만들며 `chore(manifests): bump <app> to <short-sha>` 형식을 따른다.

## 커밋 단위

- 하나의 커밋은 **하나의 논리적 변경**이다. 리팩터링과 기능 추가를 섞지 않는다.
- 빌드/테스트가 깨진 상태로 커밋하지 않는다.
- 단계(PLAN.md의 단계 0~6) 단위로 PR을 만들고, 단계 완료 시 태그를 남기는 것을 권장한다 (예: `stage-1-gitops`).

## 브랜치와 PR

- `main`은 항상 배포 가능한 상태로 유지한다. GitHub에서 branch protection(직접 push 금지, PR 필수)을 켜는 것을 권장한다.
- 작업 브랜치: `feat/<scope>-<desc>`, `fix/<scope>-<desc>`, `docs/<desc>`, `chore/<desc>`
- PR로 병합하고 squash merge를 기본으로 한다. PR 제목도 Conventional Commits 형식을 따른다.
- 이 프로젝트는 "PR 머지 = 배포"인 GitOps이므로, 매니페스트 저장소의 머지는 곧 클러스터 변경이다. **머지 전에 diff를 반드시 확인**한다 (특히 네임스페이스, 클러스터 범위 리소스).

## 커밋 전 체크리스트 (public repo)

```bash
git status
git diff --cached                      # 의도한 변경만 스테이징되었는지
# 민감 정보 간이 점검 (내부 IP, 키, 토큰, 비밀번호)
git diff --cached | grep -nE '(^|[^0-9])10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}|BEGIN [A-Z ]*PRIVATE KEY|token|password|secret|apikey' || echo "no match"
```

- kubeconfig, `.env`, 키/인증서가 스테이징되지 않았는지 확인 (`.gitignore`에 등록되어 있음)
- 가능하면 **gitleaks**를 pre-commit 훅이나 CI에 추가한다 (단계 1의 CI 작업에서 도입 권장)
- 내부 IP와 호스트명은 `<CLUSTER_IP>`, `<INGRESS_HOST>` 같은 플레이스홀더로 쓴다

## 커밋/푸시 절차 (사용자 직접 실행)

> ⚠️ **이 섹션의 모든 명령은 사용자가 직접 입력하는 명령이다. Claude는 이 섹션의 명령을 절대 실행하지 않는다.**
> (`git add`, `git commit`, `git push`, `git checkout/switch`, `git merge`, `git rebase`, `git reset`, `git tag`, `gh pr ...` 등 저장소 상태나 원격을 바꾸는 모든 git 명령 포함.
> 예외는 사용자가 요청했을 때의 읽기 전용 조회(`git status`, `git diff`, `git log`)뿐이다. Claude는 아래 절차를 보고 **커밋 메시지 초안만 제안**한다.)

실행 위치는 모두 🖥️ **[맥 · 해당 저장소 경로]** 이다. 두 저장소는 각각 따로 커밋한다 (`~/projects/ops-platform`, `~/projects/ops-platform-manifests`).

### 0. 최초 1회: 커밋 작성자 정보 확인

public 저장소의 커밋에는 이메일이 그대로 공개된다. 회사 이메일 노출을 원하지 않으면 GitHub noreply 주소를 쓴다 (GitHub → Settings → Emails에서 확인).

```bash
cd ~/projects/ops-platform
git config user.name
git config user.email
# 저장소 한정으로 바꾸려면 (--global 없음):
git config user.name "<이름>"
git config user.email "<id+username@users.noreply.github.com>"
```

### 1. 작업 브랜치 만들기 (최초 부트스트랩 커밋 이후부터 권장)

```bash
cd ~/projects/ops-platform
git switch main
git pull --ff-only origin main
git switch -c feat/operator-scheduledscaler-crd     # 브랜치 이름 규칙은 위 "브랜치와 PR" 참고
```

### 2. 변경 내용 점검

```bash
git status
git diff                      # 아직 스테이징 전 변경
```

### 3. 스테이징 (파일을 명시해서 추가한다)

`git add .` 대신 파일/디렉토리를 지정해 의도하지 않은 파일(kubeconfig, .env 등)이 섞이는 것을 막는다.

```bash
git add docs/ CLAUDE.md .claude/settings.json .gitignore
git status                    # Changes to be committed 목록 확인
git diff --cached             # 스테이징된 최종 내용 확인
```

### 4. 민감 정보 점검 (통과해야 커밋)

```bash
git diff --cached | grep -nE '(^|[^0-9])10\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}|BEGIN [A-Z ]*PRIVATE KEY|token|password|secret|apikey' || echo "no match"
```

- `no match`가 아니면 해당 줄을 확인하고 플레이스홀더(`<CLUSTER_IP>` 등)로 바꾼 뒤 다시 스테이징한다.
- kubeconfig, `.env`, `*.pem`, `*.key`가 `git status`에 보이면 즉시 중단하고 `.gitignore`를 점검한다.

### 5. 커밋

```bash
git commit -m "docs: add project plan, claude rules and git conventions"
git log --oneline -3          # 방금 커밋 확인
```

### 6. 푸시

```bash
git push -u origin feat/operator-scheduledscaler-crd    # 새 브랜치 첫 push (이후는 git push)
```

최초 부트스트랩 커밋을 `main`에 바로 올릴 때는 (branch protection 설정 전에만):

```bash
git push origin main
```

### 7. PR 생성과 병합 (GitHub 웹에서)

1. GitHub 저장소 페이지의 "Compare & pull request" 클릭 (PR 제목은 Conventional Commits 형식)
2. **Files changed 탭에서 diff를 다시 확인**한다. 특히 `ops-platform-manifests`는 머지가 곧 클러스터 변경(GitOps)이므로 네임스페이스, 클러스터 범위 리소스, 이미지 태그를 꼭 본다.
3. "Squash and merge"로 병합하고 브랜치를 삭제한다.

### 8. 병합 후 로컬 정리

```bash
git switch main
git pull --ff-only origin main
git branch -d feat/operator-scheduledscaler-crd
```

### 자주 쓰는 복구 명령 (사용자 직접)

```bash
git restore --staged <파일>          # 스테이징 취소 (파일 내용은 유지)
git commit --amend                   # 방금 커밋 메시지 수정 (push 전에만)
git switch -                         # 이전 브랜치로 복귀
```

- push한 커밋에 대해 `amend`, `reset --hard`, force push는 하지 않는다.
- 시크릿을 push했다면 히스토리 정리보다 **해당 키/토큰 폐기(rotate)가 먼저**다.

## 금지

- `main`에 force push, `--no-verify`로 훅 우회
- 시크릿 커밋. **실수로 올렸다면 히스토리 삭제보다 먼저 해당 키/토큰을 즉시 폐기(rotate)한다.** public 저장소에 한 번 올라간 값은 이미 유출된 것으로 본다.
- 클러스터 내부 정보(IP, 호스트명, 계정)가 포함된 로그/출력을 이슈, PR, 커밋 메시지에 붙여넣기
