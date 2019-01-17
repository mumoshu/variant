CMD ?= $(shell pwd)/dist/$(VERSION)/var
GITHUB_USER ?= mumoshu
GITHUB_REPO ?= variant
VERSION ?= $(shell scripts/version)
IT_DIR = test/integration

define GO_FMT
test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -d {} + | tee /dev/stderr)" || \
test -z "$$(find . -path ./vendor -prune -type f -o -name '*.go' -exec gofmt -w {} + | tee /dev/stderr)"
endef

reinstall-local: dist/$(VERSION)
	if [ -f /usr/local/bin/var ]; then rm /usr/local/bin/var && cp dist/$(VERSION)/var /usr/local/bin/var; fi

install-local: /usr/local/bin/var

/usr/local/bin/var: dist/$(VERSION)
	cp dist/$(VERSION)/var /usr/local/bin/var

format:
	$(call GO_FMT)

clean:
	rm -Rf dist/$(VERSION)

release/minor:
	git fetch origin master
	if [ git branch | grep autorelease ]; then git branch -D autorelease; else echo no branch to be cleaned; fi
	git checkout -b autorelease origin/master
	hack/semtag final -s minor

release/patch:
	git fetch origin master
	if [ git branch | grep autorelease ]; then git branch -D autorelease; else echo no branch to be cleaned; fi
	git checkout -b autorelease origin/master
	hack/semtag final -s patch

.PHONY: build
build: dist/$(VERSION)

.PHONY: cross-build
cross-build:
	scripts/cross-build

dist/$(VERSION):
	$(call GO_FMT)
	mkdir -p dist/$(VERSION)
	# $ go tool nm dist/v$(VERSION)/var | grep VERSION
	#  8b0780 D _/Users/me/path/to/variant/cli/version.VERSION
	#  6dff9c R _/Users/me/path/to/variant/cli/version.VERSION.str
	go build -ldflags "-X '_$(shell pwd)/pkg/cli/version.VERSION=$(VERSION)'" -o dist/$(VERSION)/var .

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
	cd $(IT_DIR) && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./task-step-inputs-test ok1 && ./task-step-inputs-test ok2 && echo smoke11-ok passed.

smoke11-ng: build
	cd $(IT_DIR) && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && (./task-step-inputs-test ng1; [ $$? -eq 1 ]) && echo smoke11-ng passed.

smoke12: build
	cd $(IT_DIR)/override-with-empty && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./test --logtostderr test > out && cat out | tee /dev/stderr | grep -v "bar" && echo smoke12 passed.

smoke13: build
	cd $(IT_DIR)/override-with-null && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./test --logtostderr test > out && cat out | tee /dev/stderr | grep "bar" && echo smoke13 passed.

smoke14: build
	cd $(IT_DIR)/override-with-null-from-env-config && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./test --logtostderr test > out && cat out | tee /dev/stderr | grep "baz" && echo smoke14 passed.

smoke15: build
	cd $(IT_DIR) && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./params-and-opts --logtostderr test > out && cat out | tee /dev/stderr | grep "param1=myparam1 param2=myparam2 opt1=myopt1" && echo smoke15 passed.

smoke16: build
	cd $(IT_DIR) && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./input-validation run param1x param2x --opt_str_1=optstr1 --opt_bool_1=true --opt_int_1=10 --logtostderr > out && cat out | tee /dev/stderr | grep "param1=param1x param2=param2x opt_str_1=optstr1 opt_str_2=opt2_default opt_bool_1=true opt_bool_2=true opt_int_1=10 opt_int_2=100" && echo smoke16 passed.

smoke17: build
	cd $(IT_DIR) && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./array-input test --logtostderr > out && cat out | tee /dev/stderr | ( grep "foo: foo/foo1.txt" out && grep "foo: foo/foo2.txt" && grep "bar: bar/bar1.txt" out && grep "bar: bar/bar2.txt" out) && echo smoke17 passed.

smoke18: build
	cd $(IT_DIR) && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./value-preference file --logtostderr > file.out && ./value-preference commandline --foo=commandline_foo --logtostderr > commandline.out && ./value-preference default --logtostderr > default.out && cat file.out commandline.out default.out | tee /dev/stderr | ( grep "file.foo=file_foo" file.out && grep "default.foo=default_foo" default.out && grep "commandline.foo=commandline_foo" commandline.out) && echo smoke18 passed.

smoke19: build
	cd $(IT_DIR) && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./containerized-task-dep test --logtostderr > file.out && cat file.out | tee /dev/stderr | (grep "unit=FOO" file.out) && echo smoke19 passed.

smoke20: build
	cd $(IT_DIR) && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./codebuild test --logtostderr > file.out && cat file.out | tee /dev/stderr | (grep "unit=FOO" file.out) && echo smoke20 passed.

smoke21: build
	cd examples/codebuild; make build
	cd $(IT_DIR) && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./codebuild-s3 test --s3bucket $(VARIANT_ARTIFACTS_S3_BUCKET) --logtostderr > file.out && cat file.out | tee /dev/stderr | (grep "unit=TESTDATA" file.out) && echo smoke21 passed.

smoke22: build
	cd $(IT_DIR) && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./containerized-task-autoenv test --logtostderr && echo smoke22 passed.

smoke23: build
	cd $(IT_DIR) && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./multistage-task-parameters test --logtostderr | grep foo && echo smoke23 passed.

smoke24: build
	cd $(IT_DIR) && export PATH=$(shell pwd)/dist/$(VERSION):$$PATH && ./params-type-autoenv test --logtostderr && echo smoke24 passed.

smoke-tests:
	make smoke{1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24}
