GITHUB_USER ?= mumoshu
GITHUB_REPO ?= logrus-bunyan-formatter
VERSION ?= v0.9.0

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
