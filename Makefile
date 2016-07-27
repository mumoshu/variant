gofmt:	
	test -z "$$(find . -path ./ -prune -type f -o -name '*.go' -exec gofmt -d {} + | tee /dev/stderr)" || \
	test -z "$$(find . -path ./ -prune -type f -o -name '*.go' -exec gofmt -w {} + | tee /dev/stderr)"

build:
	go build .

smoke1: build
	./dcli -v web deploy foo

smoke2: build
	./dcli web deploy foo
