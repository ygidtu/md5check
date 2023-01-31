flags="-s -w -X main.buildStamp=`date -u '+%Y-%m-%d_%I:%M:%S%p'` -X main.gitHash=`git rev-parse HEAD` -X 'main.goVersion=`go version`' -X main.version=v0.0.2"

env GOOS=windows GOARCH=amd64 go build -ldflags "$flags" -x -o md5check_win . # && upx -9 md5check_win
env GOOS=linux GOARCH=amd64 go build -ldflags "$flags" -x -o md5check_linux . # && upx -9 md5check_linux
env GOOS=darwin GOARCH=amd64 go build -ldflags "$flags" -x -o md5check_osx . # && upx -9 md5check_osx
