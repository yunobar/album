#!/bin/sh

echo "Running pre-push checks..."

echo "\n=== Running linting ==="
if ! make lint; then
    echo "❌ Linting failed! Please fix the issues before pushing."
    exit 1
fi

echo "\n=== Running tests ==="
if ! make test; then
    echo "❌ Tests failed! Please fix the test issues before pushing."
    exit 1
fi

echo "\n=== Running build ==="
if ! make build-all; then
    echo "❌ Build failed! Please fix the build issues before pushing."
    exit 1
fi

echo "\n✅ All checks passed! Pushing can continue...\n"
