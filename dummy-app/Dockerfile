# 1단계: 빌드. JDK와 Gradle이 필요하지만 최종 이미지에는 넣지 않는다.
FROM eclipse-temurin:21-jdk AS build
WORKDIR /src

# 의존성 정의만 먼저 복사해 두면, 소스만 바뀐 빌드에서 의존성 다운로드 레이어를 재사용한다.
COPY gradlew settings.gradle.kts build.gradle.kts ./
COPY gradle gradle
RUN ./gradlew --no-daemon dependencies > /dev/null

COPY src src
RUN ./gradlew --no-daemon bootJar -x test

# 2단계: 실행. JRE만 쓰고 root가 아닌 사용자로 실행한다.
FROM eclipse-temurin:21-jre
RUN useradd --system --uid 10001 app
WORKDIR /app
COPY --from=build /src/build/libs/*.jar app.jar
USER 10001

EXPOSE 8080
# 컨테이너 메모리 limit을 인식해 힙을 자동으로 정한다 (limit의 75%).
ENTRYPOINT ["java", "-XX:MaxRAMPercentage=75", "-jar", "/app/app.jar"]
