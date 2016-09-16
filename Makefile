CMD ?= dist/$(VERSION)/var
GITHUB_USER ?= mumoshu
GITHUB_REPO ?= variant
VERSION ?= v0.0.3-rc.2

define GO_FMT
test -z "$$(find . -path ./ -prune -type f -o -name '*.go' -exec gofmt -d {} + | tee /dev/stderr)" || \
test -z "$$(find . -path ./ -prune -type f -o -name '*.go' -exec gofmt -w {} + | tee /dev/stderr)"
endef

reinstall-local: dist/$(VERSION)
	if [ -f /usr/local/bin/var ]; then rm /usr/local/bin/var && cp dist/$(VERSION)/var /usr/local/bin/var; fi

install-local: /usr/local/bin/var

/usr/local/bin/var: dist/$(VERSION)
	cp dist/$(VERSION)/var /usr/local/bin/var

gofmt:	
	$(call GO_FMT)

clean:
	rm -Rf dist/$(VERSION)

build: dist/$(VERSION)

dist/$(VERSION):
	$(call GO_FMT)
	mkdir -p dist/$(VERSION)
	go build -o dist/$(VERSION)/var

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

smoke6: build
	VARFILE=var.definition.v3.yaml $(CMD) foo bar --message foo

smoke7: build
	$(CMD) env set dev && $(CMD) test2

test:
	make smoke{1,2,3,4,5,6,7}
