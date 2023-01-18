flags="-X main.buildStamp=`date -u '+%Y-%m-%d_%I:%M:%S%p'` -X main.gitHash=`git rev-parse HEAD` -X 'main.goVersion=`go version`' -X main.version=v0.0.1"

env GOOS=linux GOARCH=amd64 go build -ldflags "$flags" -x -o md5check . && upx -9 md5check