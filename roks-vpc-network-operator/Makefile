CONTAINER_ENGINE ?= podman
IMG ?= icr.io/roks/vpc-network-operator:latest
BFF_IMG ?= icr.io/roks/vpc-bff:latest
PLUGIN_IMG ?= icr.io/roks/vpc-network-console-plugin:latest

.PHONY: build test test-unit test-integration docker-build docker-build-bff docker-build-plugin docker-push deploy generate fmt vet lint clean

build: fmt vet
	go build -o bin/manager cmd/manager/main.go

test: test-unit

test-unit:
	go test ./... -coverprofile cover.out -count=1

test-integration:
	go test ./... -tags integration -coverprofile cover-integration.out -count=1 -timeout 10m

test-cover: test-unit
	go tool cover -func=cover.out

docker-build:
	$(CONTAINER_ENGINE) build -t $(IMG) .

docker-build-bff:
	$(CONTAINER_ENGINE) build -t $(BFF_IMG) -f cmd/bff/Dockerfile .

docker-build-plugin:
	$(CONTAINER_ENGINE) build -t $(PLUGIN_IMG) ../console-plugin/

docker-push:
	$(CONTAINER_ENGINE) push $(IMG)

deploy:
	helm upgrade --install roks-vpc-network-operator deploy/helm/roks-vpc-network-operator/ \
		--namespace roks-vpc-network-operator --create-namespace

helm-lint:
	helm lint deploy/helm/roks-vpc-network-operator/

helm-template:
	helm template test deploy/helm/roks-vpc-network-operator/ --set cluster.id=test --set cluster.region=us-south

generate:
	controller-gen object paths="./api/..."

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/ cover.out cover-integration.out
