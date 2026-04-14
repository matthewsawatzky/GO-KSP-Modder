# build.sh
#!/bin/bash
mkdir -p dist

GOOS=linux   GOARCH=amd64  go build -o dist/app-linux-amd64    .
GOOS=linux   GOARCH=arm64  go build -o dist/app-linux-arm64    .
GOOS=darwin  GOARCH=arm64  go build -o dist/app-macos-arm64    .
GOOS=darwin  GOARCH=amd64  go build -o dist/app-macos-x86_64   .
GOOS=windows GOARCH=amd64  go build -o dist/app-windows.exe    .

echo "Done:"
ls -lh dist/
