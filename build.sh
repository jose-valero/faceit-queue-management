#!/bin/bash
set -e

OUTPUT_DIR=${OUTPUT_DIR:-./dist/bot}

echo "Building bot application..."
rm -rf $OUTPUT_DIR && mkdir -p $OUTPUT_DIR
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o $OUTPUT_DIR/bot ./cmd/bot/main.go

echo "Build complete: $OUTPUT_DIR/bot"
