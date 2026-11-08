GO ?= go
GOFMT ?= gofmt
IMAGE ?= unifi-os-backup:local

.PHONY: check fmt-check image image-test

check: fmt-check
	$(GO) vet ./...
	$(GO) test ./...
	$(GO) test -race ./...

fmt-check:
	@unformatted=$$($(GOFMT) -l .); if [ -n "$$unformatted" ]; then printf '%s\n' "$$unformatted"; exit 1; fi

image:
	docker build -t $(IMAGE) .

image-test: image
	scripts/image-test.sh $(IMAGE)
