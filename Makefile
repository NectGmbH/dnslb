VERSION=0.1.0
IMAGE=kavatech/dnslb:$(VERSION)
BUILD_FLAGS="-X main.APPVERSION=$(VERSION)"

build:
	go build -ldflags $(BUILD_FLAGS)

build-linux:
	GOOS=linux go build -ldflags $(BUILD_FLAGS)

docker-test:
	go test -v 

docker-build: build-linux docker-test
	docker build -t $(IMAGE) .

docker-push: docker-build
	docker push $(IMAGE)
