CMD ?= $(shell pwd)/dist/$(VERSION)/var
GITHUB_USER ?= mumoshu
GITHUB_REPO ?= variant
VERSION ?= 0.0.8
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
	# $ go tool nm dist/v$(VERSION)/var | grep VERSION
	#  8b0780 D _/Users/me/path/to/variant/cli/version.VERSION
	#  6dff9c R _/Users/me/path/to/variant/cli/version.VERSION.str
	go build -ldflags "-X '_$(shell pwd)/cli/version.VERSION=$(VERSION)'" -o dist/$(VERSION)/var .

release: dist/$(VERSION)
	ghr -u $(GITHUB_USER) -r $(GITHUB_REPO) -c master --prerelease v$(VERSION) dist/$(VERSION)

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

smoke10: smoke10-ok smoke10-ng

smoke10-ok: build
	cd $(IT_DIR) && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./if-test ok1 && ./if-test ok2 && ./if-test ok3 && ./if-test ok4 && ./if-test ok5 && echo smoke10-ok passed.

smoke10-ng: build
	cd $(IT_DIR) && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && (./if-test ng1; [ $$? -eq 1 ]) && (./if-test ng2; [ $$? -eq 1 ]) && echo smoke10-ng passed.

smoke11: smoke11-ok smoke11-ng

smoke11-ok: build
	cd $(IT_DIR) && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./flow-step-inputs-test ok1 && ./flow-step-inputs-test ok2 && echo smoke11-ok passed.

smoke11-ng: build
	cd $(IT_DIR) && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && (./flow-step-inputs-test ng1; [ $$? -eq 1 ]) && echo smoke11-ng passed.

smoke12: build
	cd $(IT_DIR)/override-with-empty && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./test --logtostderr test > out && cat out | tee /dev/stderr | grep -v "bar" && echo smoke12 passed.

smoke13: build
	cd $(IT_DIR)/override-with-null && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./test --logtostderr test > out && cat out | tee /dev/stderr | grep "bar" && echo smoke13 passed.

smoke14: build
	cd $(IT_DIR)/override-with-null-from-env-config && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./test --logtostderr test > out && cat out | tee /dev/stderr | grep "baz" && echo smoke14 passed.

smoke-tests:
	make smoke{1,2,3,4,5,6,7,8,9,10,11,12,13,14}
