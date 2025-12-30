#!/bin/bash

# Integration test for skool-downloader with yt-dlp in Docker
# This script builds the Docker image and tests basic functionality

set -e

echo "🔨 Building Docker image..."
docker build -t skool-downloader:test .

echo ""
echo "✅ Docker build successful!"
echo ""
echo "🧪 Running integration tests..."
echo ""

# Test 1: Check that the binary exists and is executable
echo "Test 1: Verify binary is executable in container..."
docker run --rm skool-downloader:test --help > /dev/null 2>&1 && echo "✅ Binary is executable" || (echo "❌ Binary not executable" && exit 1)

# Test 2: Check that yt-dlp is installed and accessible
echo "Test 2: Verify yt-dlp is installed..."
docker run --rm --entrypoint sh skool-downloader:test -c "which yt-dlp" > /dev/null 2>&1 && echo "✅ yt-dlp is installed" || (echo "❌ yt-dlp not found" && exit 1)

# Test 3: Check that yt-dlp can run
echo "Test 3: Verify yt-dlp can execute..."
docker run --rm --entrypoint sh skool-downloader:test -c "yt-dlp --version" > /dev/null 2>&1 && echo "✅ yt-dlp is functional" || (echo "❌ yt-dlp cannot execute" && exit 1)

# Test 4: Check that chromium is installed
echo "Test 4: Verify chromium is installed..."
docker run --rm --entrypoint sh skool-downloader:test -c "which chromium-browser || which chromium" > /dev/null 2>&1 && echo "✅ Chromium is installed" || (echo "❌ Chromium not found" && exit 1)

# Test 5: Check that ffmpeg is installed
echo "Test 5: Verify ffmpeg is installed..."
docker run --rm --entrypoint sh skool-downloader:test -c "which ffmpeg" > /dev/null 2>&1 && echo "✅ ffmpeg is installed" || (echo "❌ ffmpeg not found" && exit 1)

# Test 6: Test that binary shows proper error without arguments
echo "Test 6: Verify proper error handling without arguments..."
docker run --rm skool-downloader:test 2>&1 | grep -q "Usage:" && echo "✅ Proper error message displayed" || (echo "❌ Error message not as expected" && exit 1)

# Test 7: Test that binary accepts help flag
echo "Test 7: Verify help flag works..."
docker run --rm skool-downloader:test --help 2>&1 | grep -q "url" && echo "✅ Help flag works" || (echo "❌ Help flag doesn't work" && exit 1)

echo ""
echo "🎉 All integration tests passed!"
echo ""
