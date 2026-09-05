.PHONY: build

build:
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./build/quill ./cmd/quill/main.go
	@echo "Binary built in 'build' directory"


run:
	go run ./cmd/quill/main.go
