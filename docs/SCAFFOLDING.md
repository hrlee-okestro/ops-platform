# 저장소 골격 생성 기록 (단계 0)

각 모듈의 초기 골격을 어떤 명령으로 만들었는지 기록한다. 읽기만 해도 흐름을 이해할 수 있고, 같은 명령으로 직접 다시 만들어볼 수 있다.

모든 명령의 실행 위치는 🖥️ **[맥 · 저장소 경로: `~/projects/ops-platform`]** 이다.

## 사전 조건 (설치 확인)

| 도구 | 확인 명령 | 사용한 버전 |
|---|---|---|
| Go | `go version` | 1.27.3 |
| Node | `node -v` | 22.23.2 |
| JDK | `/usr/libexec/java_home -V` | 21 (Microsoft 21.0.10). 맥 기본값은 17이라 빌드 시 지정이 필요하다 |
| kubebuilder | `kubebuilder version` | 4.16.0 |

JDK 21을 셸에서 쓰려면 다음과 같이 한다.

```bash
export JAVA_HOME=$(/usr/libexec/java_home -v 21)
```

## 1. operator (Go + kubebuilder)

```bash
mkdir operator && cd operator

# 프로젝트 초기화: go.mod, Makefile, Dockerfile, cmd/main.go, config/ 생성
kubebuilder init --domain okestro.dev --repo github.com/hrlee-okestro/ops-platform/operator --plugins go/v4

# ScheduledScaler 리소스(CRD 타입)와 컨트롤러 뼈대 생성
kubebuilder create api --group ops --version v1alpha1 --kind ScheduledScaler --resource --controller

# CRD/RBAC YAML(config/)과 DeepCopy 코드(zz_generated.deepcopy.go) 생성
make manifests generate

# 빌드 확인
go build ./...
```

- `--domain`과 `--group`이 합쳐져 CRD의 API 그룹이 `ops.okestro.dev`가 된다.
- `--repo`는 Go 모듈 경로다. `go.mod`의 `module` 값으로 들어간다.
- `kubebuilder init` 중 `go mod tidy`가 실행되어 필요한 모듈(controller-runtime, client-go 등)을 자동으로 내려받는다. 첫 실행은 시간이 걸린다.
- `make manifests generate` 때 `bin/controller-gen`을 내려받는다 (`.gitignore`로 제외됨).

주요 파일:

| 파일 | 역할 |
|---|---|
| `api/v1alpha1/scheduledscaler_types.go` | CRD의 spec/status 정의 (직접 채워 넣을 곳) |
| `internal/controller/scheduledscaler_controller.go` | `Reconcile()`이 비어 있음. reconcile loop 본체 |
| `cmd/main.go` | 매니저 시작점. 메트릭, 헬스체크 설정 |
| `config/crd`, `config/rbac` | 생성되는 CRD와 RBAC YAML |
| `PROJECT` | kubebuilder 메타데이터. 직접 수정하지 않는다 |

참고: kubebuilder가 `operator/.github/workflows/`에 CI 파일을 만든다. GitHub Actions는 저장소 루트의 `.github/`만 읽으므로 이 파일들은 동작하지 않는다.

## 2. console-web (React + TypeScript, Vite)

```bash
# 템플릿 생성 (react-ts)
npm create vite@latest console-web -- --template react-ts

cd console-web
npm install        # package.json의 의존성을 node_modules/에 설치
npm run build      # tsc 타입 체크 후 dist/ 생성 (빌드 확인)
```

- 개발 서버는 `npm run dev`로 띄운다 (기본 포트 5173).
- 주요 파일: `src/main.tsx`(진입점), `src/App.tsx`(템플릿 화면), `vite.config.ts`, `tsconfig*.json`.
- `node_modules/`와 `dist/`는 `.gitignore`로 제외된다.

## 3. console-api, dummy-app (Spring Boot, Gradle)

Spring Initializr(start.spring.io)를 웹 UI 대신 HTTP로 호출해 프로젝트를 내려받았다. 웹 UI에서 같은 값을 골라 내려받아도 결과는 같다.

```bash
BASE="https://start.spring.io/starter.tgz?type=gradle-project-kotlin&language=java&javaVersion=21&packaging=jar&groupId=dev.okestro.ops"

mkdir console-api dummy-app

# console-api
curl -s "$BASE&name=console-api&artifactId=console-api&baseDir=.&dependencies=web,data-jpa,postgresql,flyway,security,actuator,prometheus,validation" -o console-api.tgz
tar -xzf console-api.tgz -C console-api && rm console-api.tgz

# dummy-app
curl -s "$BASE&name=dummy-app&artifactId=dummy-app&baseDir=.&dependencies=web,actuator,prometheus" -o dummy-app.tgz
tar -xzf dummy-app.tgz -C dummy-app && rm dummy-app.tgz

# 압축 해제 후 gradlew 실행 권한이 없어서 부여
chmod +x console-api/gradlew dummy-app/gradlew

# 빌드 확인 (JDK 21 필요, 테스트는 제외)
export JAVA_HOME=$(/usr/libexec/java_home -v 21)
(cd console-api && ./gradlew build -x test)
(cd dummy-app && ./gradlew build -x test)
```

- Spring Boot 버전을 지정하지 않으면 Initializr 기본값(현재 4.1.x)이 쓰인다. 3.x는 Initializr가 더 이상 받아주지 않아 4.x로 진행했다.
- Kotlin DSL(`build.gradle.kts`)을 선택했다. 첫 `./gradlew` 실행 때 Gradle 배포판과 의존성을 내려받는다.
- 의존성은 `build.gradle.kts`의 `dependencies { }`에서 확인한다.
- 진입점은 `src/main/java/dev/okestro/ops/.../*Application.java`, 설정은 `src/main/resources/application.properties`다.
- `-x test`로 테스트를 제외한 이유: console-api는 JPA, Flyway, PostgreSQL 의존성이 있어 DB 연결 설정 없이는 컨텍스트 로딩 테스트가 실패할 수 있다. DB 설정은 단계 3에서 다룬다.
- `build/`, `.gradle/`은 `.gitignore`로 제외된다.

## 빌드 결과 요약

| 모듈 | 빌드 명령 | 결과 |
|---|---|---|
| operator | `go build ./...` | 성공 |
| console-web | `npm run build` | 성공 |
| console-api | `./gradlew build -x test` | 성공 |
| dummy-app | `./gradlew build -x test` | 성공 |
