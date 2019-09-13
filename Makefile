COMMIT=$(shell git rev-parse HEAD | head -c 8)$(shell if ! git diff --no-ext-diff --quiet --exit-code; then echo .m; fi)
APP=gatekeeper
BUILD=-dev
REPO=ehazlett/$(APP)

all: build

build:
	@>&2 echo " -> building ${COMMIT}"
	@CGO_ENABLED=0 go build -installsuffix cgo -ldflags "-w -X github.com/$(REPO)/version.GitCommit=$(COMMIT) -X github.com/$(REPO)/version.Build=$(BUILD)" -o ./bin/$(APP) .

clean:
	@rm -rf bin

.PHONY: build clean
