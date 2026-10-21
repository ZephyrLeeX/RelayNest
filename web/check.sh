#!/bin/sh
set -eu
cd "$(dirname "$0")"
npm ci --no-audit --no-fund
npm run build
cd ../server
go test -race ./...
go vet ./...
test -z "$(gofmt -l cmd internal)"
