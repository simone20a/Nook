run:
	go run .
build:
	go build -o nook .
test:
	go test ./...
vet:
	go vet ./...
fmt:
	gofmt -w $$(find . -name '*.go')
