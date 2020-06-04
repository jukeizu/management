TAG=$(shell git describe --tags --always)
VERSION=$(TAG:v%=%)
REPO=jukeizu/management
GO=go
BUILD=GOARCH=amd64 $(GO) build -ldflags="-s -w -X main.Version=$(VERSION)" 
PROTOFILES=$(wildcard api/protobuf-spec/*/*.proto)
PBFILES=$(patsubst %.proto,%.pb.go, $(PROTOFILES))

.PHONY: all deps test build build-linux docker-build docker-save docker-deploy proto clean $(PROTOFILES)

all: deps test build 
deps:
	$(GO) mod download

test:
	$(GO) vet ./...
	$(GO) test -v -race ./...

build:
	$(BUILD) -o bin/management-$(VERSION) ./cmd/...

build-linux:
	CGO_ENABLED=0 GOOS=linux $(BUILD) -a -installsuffix cgo -o bin/management ./cmd/...

docker-build:
	docker build -t $(REPO):$(VERSION) .

docker-save:
	mkdir -p bin && docker save -o bin/image.tar $(REPO):$(VERSION)

docker-deploy:
	docker push $(REPO):$(VERSION)

proto: $(PBFILES)

%.pb.go: %.proto
	cd $(dir $<) && protoc $(notdir $<) --go_out=plugins=grpc:.

clean:
	@find bin -type f ! -name '*.toml' -delete -print
