gofmt:	
	test -z "$$(find . -path ./ -prune -type f -o -name '*.go' -exec gofmt -d {} + | tee /dev/stderr)" || \
	test -z "$$(find . -path ./ -prune -type f -o -name '*.go' -exec gofmt -w {} + | tee /dev/stderr)"

build: gofmt
	go build .

smoke1: build
	./dcli -v web deploy foo

smoke2: build
	./dcli web deploy foo

smoke3: build
	./dcli -v web deploy --target foo

smoke4: build
	./dcli -v add 1 2

smoke5: build
	./dcli all -v --web-deploy-target tar --job-deploy-job-id jobid
