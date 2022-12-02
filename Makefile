build:
	@go build -o bin/proxy

build_amd:
	@env GOOS=darwin GOARCH=amd64 go build -o bin/amd/proxy

build_arm:
	@env GOOS=darwin GOARCH=arm64 go build -o bin/arm/proxy

run: build
	@./bin/proxy

test:
	@go test -v ./...