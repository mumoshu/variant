CMD ?= ./var
GITHUB_USER ?= mumoshu
GITHUB_REPO ?= variant
VERSION ?= v0.0.1

gofmt:	
	test -z "$$(find . -path ./ -prune -type f -o -name '*.go' -exec gofmt -d {} + | tee /dev/stderr)" || \
	test -z "$$(find . -path ./ -prune -type f -o -name '*.go' -exec gofmt -w {} + | tee /dev/stderr)"

build: gofmt
	go build -o $(CMD) .

dist/$(VERSION): build
	mkdir -p dist/$(VERSION)
	cp $(CMD) dist/$(VERSION)/

release: dist/$(VERSION)
	ghr -u $(GITHUB_USER) -r $(GITHUB_REPO) -c master --prerelease $(VERSION) dist/$(VERSION)

publish-latest: dist/$(VERSION)
	ghr -u $(GITHUB_USER) -r $(GITHUB_REPO) -c master --replace --prerelease latest dist/$(VERSION)

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
