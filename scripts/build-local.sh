#!/bin/sh
# Build on the host, then package with cached runtime images when registry builder pulls stall.
set -eu
ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
cd "$ROOT"
GO_BIN=${GO_BIN:-go}
IMAGE_PREFIX=${IMAGE_PREFIX:-cvd-secs}
ARCH=${TARGET_ARCH:-$(docker info --format '{{.Architecture}}')}
case "$ARCH" in aarch64|arm64) ARCH=arm64; JRE_ARCH=aarch64;; x86_64|amd64) ARCH=amd64; JRE_ARCH=x64;; *) echo "Unsupported architecture: $ARCH" >&2; exit 1;; esac
mkdir -p .runtime/images/simulator .runtime/images/gateway .runtime/images/backend .runtime/images/frontend .runtime/images/bridge
(cd simulator && CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" "$GO_BIN" build -o ../.runtime/images/simulator/app .)
(cd gateway && CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" "$GO_BIN" build -o ../.runtime/images/gateway/app .)
(cd bridge && CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" "$GO_BIN" build -o ../.runtime/images/bridge/app .)
(cd backend && mvn -B package)
(cd frontend && npm ci && npm run build)
cp backend/target/cvd-0.1.0.jar .runtime/images/backend/app.jar
cp -R frontend/dist .runtime/images/frontend/
cp frontend/nginx.conf .runtime/images/frontend/
if [ ! -x .runtime/images/backend/jre/bin/java ]; then
 curl -fL --retry 2 "https://api.adoptium.net/v3/binary/version/jdk-21.0.12.1%2B1/alpine-linux/$JRE_ARCH/jre/hotspot/normal/eclipse" -o .runtime/jre.tar.gz
 mkdir -p .runtime/images/backend/jre
 tar -xzf .runtime/jre.tar.gz --strip-components=1 -C .runtime/images/backend/jre
fi
for service in simulator gateway bridge; do
 cat > ".runtime/images/$service/Dockerfile" <<'DOCKER'
FROM alpine:3.22
COPY app /app
ENV DATA_DIR=/data
ENTRYPOINT ["/app"]
DOCKER
done
cat > .runtime/images/backend/Dockerfile <<'DOCKER'
FROM alpine:3.22
COPY jre /opt/java
COPY app.jar /app.jar
ENV PATH=/opt/java/bin:$PATH
ENTRYPOINT ["java","-jar","/app.jar"]
DOCKER
cat > .runtime/images/frontend/Dockerfile <<'DOCKER'
FROM nginx:1.27-alpine
COPY dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
DOCKER
for service in simulator gateway bridge backend frontend; do
 docker build --pull=false -t "$IMAGE_PREFIX-$service" ".runtime/images/$service"
done
