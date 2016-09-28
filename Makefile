CMD ?= $(shell pwd)/dist/$(VERSION)/var
GITHUB_USER ?= mumoshu
GITHUB_REPO ?= variant
VERSION ?= v0.0.5
IT_DIR = test/integration

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
	cd $(IT_DIR) && $(CMD) -v web deploy foo

smoke2: build
	cd $(IT_DIR) && $(CMD) web deploy foo

smoke3: build
	cd $(IT_DIR) && $(CMD) -v web deploy --target foo

smoke4: build
	cd $(IT_DIR) && $(CMD) -v add 1 2

smoke5: build
	cd $(IT_DIR) && $(CMD) all -v --web-deploy-target tar --job-deploy-job-id jobid

smoke6: build
	cd $(IT_DIR) && VARFILE=var.definition.v3.yaml $(CMD) foo bar --message foo

smoke7: build
	cd $(IT_DIR) && $(CMD) env set dev && $(CMD) test2

smoke8: build
	cd $(IT_DIR) && PATH=$(shell pwd)/dist/$(VERSION):$$PATH ./steps-test ok && echo smoke8 passed.

smoke9: build
	cd $(IT_DIR) && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./or-step-test ok && (./or-step-test ng; [ $$? -eq 1 ]) && echo smoke9 passed.

smoke10: build
	cd $(IT_DIR) && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./if-test ok1 && ./if-test ok2 && ./if-test ok3 && (./if-test ng1; [ $$? -eq 1 ]) && (./if-test ng2; [ $$? -eq 1 ]) && echo smoke10 passed.

smoke-tests:
	make smoke{1,2,3,4,5,6,7,8,9,10}
