CMD := ./var

gofmt:	
	test -z "$$(find . -path ./ -prune -type f -o -name '*.go' -exec gofmt -d {} + | tee /dev/stderr)" || \
	test -z "$$(find . -path ./ -prune -type f -o -name '*.go' -exec gofmt -w {} + | tee /dev/stderr)"

build: gofmt
	go build .

smoke1: build
	$(CMD) -v web deploy foo

smoke2: build
	$(CMD) web deploy foo

smoke3: build
	$(CMD) -v web deploy --target foo

smoke4: build
	$(CMD) -v add 1 2

smoke5: build
	$(CMD) all -v --web-deploy-target tar --job-deploy-job-id jobid

test:
	make smoke{1,2,3,4,5}
