#!/bin/bash

# EC2 Build Script for Go Backend
# Builds Docker image with version tagging for the Go-based backend
# Usage: ./golang-dep-scripts/ec2-go-build.sh [environment] [build-number]
#   environment: dev, staging, or prod (default: dev)
#   build-number: Optional, will auto-increment if not provided

set -e

IMAGE_NAME="golang-backend"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Parse arguments
ENVIRONMENT=${1:-dev}
BUILD_NUMBER=$2

# Valid environments
VALID_ENVS=("dev" "staging" "prod")

# Validate environment
if [[ ! " ${VALID_ENVS[@]} " =~ " ${ENVIRONMENT} " ]]; then
    echo "❌ Invalid environment: $ENVIRONMENT"
    echo "   Valid environments: ${VALID_ENVS[*]}"
    exit 1
fi

echo "=========================================="
echo "Building Go Backend Docker Image"
echo "=========================================="
echo "Environment: $ENVIRONMENT"
echo ""

# Auto-increment build number if not provided
if [ -z "$BUILD_NUMBER" ]; then
    echo "Auto-detecting next build number..."

    # Get all existing build numbers for this environment (matching BUILD_NUMBER-ENVIRONMENT pattern)
    EXISTING=$(docker images "$IMAGE_NAME" --format "{{.Tag}}" --filter "dangling=false" | \
        grep -E "^[0-9]+-$ENVIRONMENT$" | \
        sed "s/-$ENVIRONMENT$//" | \
        sort -V -u -r)

    if [ -z "$EXISTING" ]; then
        BUILD_NUMBER=1
        echo "No existing images found. Starting with build number: $BUILD_NUMBER"
    else
        HIGHEST=$(echo "$EXISTING" | head -n 1)
        BUILD_NUMBER=$((HIGHEST + 1))
        echo "Highest existing build number: $HIGHEST"
        echo "Next build number: $BUILD_NUMBER"
    fi
else
    echo "Using provided build number: $BUILD_NUMBER"
fi

# Validate build number is a positive integer
if ! [[ "$BUILD_NUMBER" =~ ^[1-9][0-9]*$ ]]; then
    echo "❌ Error: Build number must be a positive integer"
    exit 1
fi

echo ""
echo "Build Configuration:"
echo "  Image: $IMAGE_NAME"
echo "  Environment: $ENVIRONMENT"
echo "  Build Number: $BUILD_NUMBER"
echo "  Tags:"
echo "    - $IMAGE_NAME:$BUILD_NUMBER-$ENVIRONMENT"
echo "    - $IMAGE_NAME:latest"
echo ""

# Change to project directory (assumes Dockerfile for Go backend is here)
cd "$PROJECT_DIR" || exit 1

# Check if Dockerfile exists
if [ ! -f "Dockerfile" ]; then
    echo "❌ Dockerfile not found in $PROJECT_DIR"
    exit 1
fi

# Clean up Docker build cache
echo "Cleaning up Docker build cache..."
docker builder prune -f
echo ""

# Build the image with all tags
echo "Building Docker image..."
echo ""

if docker build \
    -t "$IMAGE_NAME:$BUILD_NUMBER-$ENVIRONMENT" \
    -t "$IMAGE_NAME:latest" \
    .; then
    echo ""
    echo "✅ Build successful!"
    echo ""
    echo "Image tags:"
    echo "  - $IMAGE_NAME:$BUILD_NUMBER-$ENVIRONMENT"
    echo "  - $IMAGE_NAME:latest"
    echo ""
    echo "Build complete. Image ready for deployment."
    echo ""
else
    echo ""
    echo "❌ Build failed!"
    exit 1
fi
