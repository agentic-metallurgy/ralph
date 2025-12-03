.PHONY: build test clean

build:
	go build -o ralph ./cmd/ralph

test:
	go test ./...

clean:
	rm -f ralph ralph-test
