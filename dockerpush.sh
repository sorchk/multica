#!/bin/bash
name=wdjh
ver=$1
# 在代码中搜索 my-version，修改版本号
build_date=$(date +"%Y%m%d")
if [ -z "${ver}" ]; then
  ver=0.2.16
fi
echo ${ver}_${build_date}
export DOCKER_CLI_EXPERIMENTAL=enabled
export DOCKER_BUILDKIT=1
# docker login -u sorc
# docker run --privileged --rm tonistiigi/binfmt --install all
# docker buildx create --name builder --driver=docker-container --use --bootstrap
# docker buildx ls
docker buildx build \
  -f Dockerfile \
  --platform linux/arm64,linux/amd64 \
  --build-arg VERSION=${ver} \
  --build-arg NEXT_PUBLIC_APP_VERSION=${ver} \
  --build-arg BUILD_DATE=${build_date} \
  --push \
  --tag sorc/${name}:${ver}_${build_date} \
  --tag sorc/${name}:${ver} \
  --tag sorc/${name}:latest .

docker buildx build \
  -f Dockerfile.web \
  --platform linux/arm64,linux/amd64 \
  --build-arg VERSION=${ver} \
  --build-arg NEXT_PUBLIC_APP_VERSION=${ver} \
  --build-arg BUILD_DATE=${build_date} \
  --push \
  --tag sorc/${name}-ui:${ver}_${build_date} \
  --tag sorc/${name}-ui:${ver} \
  --tag sorc/${name}-ui:latest .