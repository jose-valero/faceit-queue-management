#!/bin/bash

# Get current commit hash
COMMIT_HASH=$(git rev-parse --short HEAD)
ECR_REPO="506636091874.dkr.ecr.us-east-1.amazonaws.com/dev-faceit-cluster-app"

# Build Go app for ARM64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o app ./cmd/bot

# Login to ECR
aws ecr get-login-password --region us-east-1 --profile cpx-valero | docker login --username AWS --password-stdin $ECR_REPO

# Build Docker image using ECS Dockerfile
docker build -f Dockerfile-ecs -t $ECR_REPO:$COMMIT_HASH .

# Push to ECR
AWS_PROFILE=cpx-valero docker push $ECR_REPO:$COMMIT_HASH

# Clean up binary
rm app

echo "Pushed image: $ECR_REPO:$COMMIT_HASH"