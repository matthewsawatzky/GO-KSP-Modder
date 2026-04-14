# build.sh
#!/bin/bash
rm -r dist 2>/dev/null || true
mkdir -p dist

GOOS=linux   GOARCH=amd64  go build -C go -o ../dist/app-linux-amd64    .
GOOS=linux   GOARCH=arm64  go build -C go -o ../dist/app-linux-arm64    .
GOOS=darwin  GOARCH=arm64  go build -C go -o ../dist/app-macos-arm64    .
GOOS=darwin  GOARCH=amd64  go build -C go -o ../dist/app-macos-x86_64   .
GOOS=windows GOARCH=amd64  go build -C go -o ../dist/app-windows.exe    .

echo "Done:"
ls -lh dist/