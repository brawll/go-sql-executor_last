#!/bin/bash

# EC2 Deployment Script for Go Backend
# Orchestrates build and deployment of the Go-based backend on EC2
# Usage: ./golang-dep-scripts/ec2-go-deploy.sh [environment] [build-number] [--rollback [version]]
#   environment: dev, staging, or prod (default: dev)
#   build-number: Optional, will auto-increment if not provided
#   --rollback [version]: Rollback to a specific version

set -e

IMAGE_NAME="golang-backend"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DEPLOY_DIR="$HOME/golang-backend-deploy"
HEALTH_CHECK_URL="http://localhost:8080/health"
HEALTH_CHECK_TIMEOUT=30
COMPOSE_FILE="docker-compose.go.prod.yml"

# Parse arguments
ENVIRONMENT=""
BUILD_NUMBER=""
ROLLBACK_VERSION=""
ROLLBACK=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --rollback)
            ROLLBACK=true
            ROLLBACK_VERSION=$2
            shift 2
            ;;
        dev|staging|prod)
            ENVIRONMENT=$1
            shift
            ;;
        [0-9]*)
            BUILD_NUMBER=$1
            shift
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Default environment
if [ -z "$ENVIRONMENT" ]; then
    ENVIRONMENT="dev"
fi

# Valid environments
VALID_ENVS=("dev" "staging" "prod")

# Validate environment
if [[ ! " ${VALID_ENVS[@]} " =~ " ${ENVIRONMENT} " ]]; then
    echo "❌ Invalid environment: $ENVIRONMENT"
    echo "   Valid environments: ${VALID_ENVS[*]}"
    exit 1
fi

echo "=========================================="
echo "Go Backend Deployment"
echo "=========================================="
echo "Environment: $ENVIRONMENT"
echo "Mode: $([ "$ROLLBACK" = true ] && echo "Rollback" || echo "Deploy")"
echo ""

# Reuse existing environment configuration from main backend project
ENV_CONFIG_SCRIPT="$PROJECT_ROOT/scripts/ec2-env-config.sh"
if [ ! -f "$ENV_CONFIG_SCRIPT" ]; then
    echo "❌ Environment config script not found: $ENV_CONFIG_SCRIPT"
    exit 1
fi

# Initialize config if needed
if [ ! -f "$HOME/.digiice/backend-env-config.json" ]; then
    "$ENV_CONFIG_SCRIPT" init
fi

# Get database credentials (same DB as main backend)
DB_USER=$("$ENV_CONFIG_SCRIPT" get-db-user "$ENVIRONMENT" 2>/dev/null || echo "digiice_user")
DB_PASSWORD=$("$ENV_CONFIG_SCRIPT" get-db-password "$ENVIRONMENT" 2>/dev/null || echo "digiice_password")
DB_NAME=$("$ENV_CONFIG_SCRIPT" get-db-name "$ENVIRONMENT" 2>/dev/null || echo "digiice")

# Construct DATABASE_URL if not already set
if [ -z "$DATABASE_URL" ]; then
    DB_HOST="${DB_HOST:-postgres}"
    DB_PORT="${DB_PORT:-5432}"
    DATABASE_URL="postgresql://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}"
fi

# Export environment variables for docker-compose
export BUILD_NUMBER
export ENVIRONMENT
export DB_USER
export DB_PASSWORD
export DB_NAME
export DB_HOST
export DB_PORT
export DATABASE_URL

# Rollback logic
if [ "$ROLLBACK" = true ]; then
    echo "🔄 Rolling back to version: ${ROLLBACK_VERSION:-previous}"
    echo ""

    if [ -z "$ROLLBACK_VERSION" ]; then
        # Find the previous version (second highest BUILD_NUMBER-ENVIRONMENT tag)
        PREVIOUS=$(docker images "$IMAGE_NAME" --format "{{.Tag}}" --filter "dangling=false" | \
            grep -E "^[0-9]+-$ENVIRONMENT$" | \
            sed "s/-$ENVIRONMENT$//" | \
            sort -V -r | \
            sed -n '2p')

        if [ -z "$PREVIOUS" ]; then
            echo "❌ No previous version found for rollback"
            exit 1
        fi

        ROLLBACK_VERSION=$PREVIOUS
        echo "Using previous version: $ROLLBACK_VERSION"
    fi

    export BUILD_NUMBER="$ROLLBACK_VERSION"
    DEPLOY_TAG="$ROLLBACK_VERSION-$ENVIRONMENT"

    # Verify the image exists
    if ! docker images "$IMAGE_NAME:$DEPLOY_TAG" --format "{{.Tag}}" | grep -q "$DEPLOY_TAG"; then
        echo "❌ Version $ROLLBACK_VERSION-$ENVIRONMENT not found"
        exit 1
    fi
else
    # Normal deployment - check if code exists in deploy directory
    if [ ! -d "$DEPLOY_DIR" ] || [ -z "$(ls -A "$DEPLOY_DIR" 2>/dev/null)" ]; then
        echo "❌ Deployment directory is empty or doesn't exist: $DEPLOY_DIR"
        echo "   Code should be transferred to this directory before deployment"
        exit 1
    fi

    # Build the image
    echo "Building Docker image for Go backend..."
    BUILD_SCRIPT="$DEPLOY_DIR/golang-dep-scripts/ec2-go-build.sh"

    if [ ! -f "$BUILD_SCRIPT" ]; then
        echo "❌ Build script not found: $BUILD_SCRIPT"
        echo "   Make sure golang-dep-scripts are transferred to EC2"
        exit 1
    fi

    # Change to deploy directory for build
    cd "$DEPLOY_DIR" || exit 1

    # Run build script
    if [ -n "$BUILD_NUMBER" ]; then
        "$BUILD_SCRIPT" "$ENVIRONMENT" "$BUILD_NUMBER"
    else
        "$BUILD_SCRIPT" "$ENVIRONMENT"
    fi

    # Get the build number from the latest BUILD_NUMBER-ENVIRONMENT tag
    LATEST_TAG=$(docker images "$IMAGE_NAME" --format "{{.Tag}}" --filter "dangling=false" | \
        grep -E "^[0-9]+-$ENVIRONMENT$" | \
        sort -V -r | \
        head -n 1)

    if [ -z "$LATEST_TAG" ]; then
        echo "❌ No image found for environment $ENVIRONMENT"
        exit 1
    fi

    # Extract build number from tag (e.g., "5-dev" -> "5")
    BUILD_NUMBER=$(echo "$LATEST_TAG" | sed -r "s/-[^-]+$//")
    DEPLOY_TAG="$BUILD_NUMBER-$ENVIRONMENT"
    export BUILD_NUMBER

    echo ""
    echo "✅ Build complete. Build number: $BUILD_NUMBER"
    echo ""
fi

# Stop existing containers for Go backend
echo "Stopping existing Go backend containers..."
cd "$DEPLOY_DIR" || exit 1

if [ -f "$COMPOSE_FILE" ]; then
    docker compose -f "$COMPOSE_FILE" down || true
    echo "✅ Old Go backend containers stopped"
else
    echo "❌ Docker compose file not found: $COMPOSE_FILE"
    exit 1
fi

echo ""

# Start new containers with docker-compose (Go backend only; assumes Postgres already running)
echo "Starting Go backend containers with docker-compose..."
echo "  Image: $IMAGE_NAME:$DEPLOY_TAG"
echo "  Environment: $ENVIRONMENT"
echo "  Database: $DB_NAME"
echo ""

if docker compose -f "$COMPOSE_FILE" up -d; then
    echo "✅ Go backend containers started successfully"
else
    echo "❌ Failed to start Go backend containers"
    exit 1
fi

echo ""

echo "Waiting for Go backend service to be ready..."
sleep 5

# Health check
echo "Performing health check on Go backend..."
HEALTH_CHECK_PASSED=false
for i in $(seq 1 $HEALTH_CHECK_TIMEOUT); do
    if curl -sf "$HEALTH_CHECK_URL" > /dev/null 2>&1; then
        HEALTH_CHECK_PASSED=true
        break
    fi
    echo -n "."
    sleep 1
done

echo ""

if [ "$HEALTH_CHECK_PASSED" = true ]; then
    echo "✅ Health check passed"
else
    echo "⚠️  Health check failed or timeout"
    echo "   Checking container status..."
    docker compose -f "$COMPOSE_FILE" ps
    echo ""
    echo "Go backend logs (last 20 lines):"
    docker compose -f "$COMPOSE_FILE" logs --tail 20
    echo ""
    echo "⚠️  Deployment completed but health check failed"
    echo "   Please verify the containers manually"
fi

echo ""

# Display deployment summary
echo "=========================================="
echo "Go Backend Deployment Summary"
echo "=========================================="
echo ""
echo "Environment: $ENVIRONMENT"
if [ "$ROLLBACK" = true ]; then
    echo "Mode: Rollback"
    echo "Version: $ROLLBACK_VERSION"
else
    echo "Mode: Deploy"
    echo "Build Number: $BUILD_NUMBER"
fi
echo "Image: $IMAGE_NAME:$DEPLOY_TAG"
echo ""
echo "Container Status:"
docker compose -f "$COMPOSE_FILE" ps
echo ""
echo "Access the Go API at (adjust if you change ports):"
echo "  http://$(curl -s ifconfig.me || echo 'YOUR_EC2_IP'):8001"
echo ""
echo "Useful commands:"
echo "  View logs: docker compose -f $COMPOSE_FILE logs -f"
echo "  Stop: docker compose -f $COMPOSE_FILE down"
echo "  Restart: docker compose -f $COMPOSE_FILE restart"
echo "  Rollback: $0 --rollback [version] $ENVIRONMENT"
echo ""

if [ "$HEALTH_CHECK_PASSED" = true ]; then
    echo "✅ Go backend deployment successful!"
else
    echo "⚠️  Go backend deployment completed with warnings"
    exit 1
fi
