#/bin/bash
set -eu
echo "============构建后端============"
cd server && go build -o bin/server ./cmd/server
cd server && go build -o bin/migrate ./cmd/migrate
echo "============构建前端============"
cd apps/web && pnpm --filter @multica/web build
echo "============构建后端镜像============"
docker build -t sorc/wdjh .
echo "============构建前端镜像============"
docker build -f ./Dockerfile.web -t sorc/wdjh-ui .
echo "============构建完成================"