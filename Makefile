.PHONY: run build

run:
	go run cmd/server/main.go

build:
	GOOS=linux GOARCH=amd64 go build -o barivara-server ./cmd/server
