#!/bin/bash
# This script builds the Docker image for the Database Intelligence Collector.

set -euo pipefail

IMAGE="$1"
TAG="$2"
PLATFORM="$3"
PUSH="$4"

echo "🐳 Building Docker image: ${IMAGE}:${TAG} for platform ${PLATFORM}"

BUILD_COMMAND="docker buildx build --platform ${PLATFORM} --tag ${IMAGE}:${TAG} --tag ${IMAGE}:latest"

if [ "${PUSH}" = "true" ]; then
  BUILD_COMMAND="${BUILD_COMMAND} --push"
fi

BUILD_COMMAND="${BUILD_COMMAND} ."

eval "${BUILD_COMMAND}"

echo "✅ Docker image built"
