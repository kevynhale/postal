GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
BUILD_IMG := postal-build$(if $(GIT_BRANCH),:$(GIT_BRANCH))

INTEGRATION_OPTS := $(if $(MAKE_DOCKER_HOST),-e "DOCKER_HOST=$(MAKE_DOCKER_HOST)", -v "/var/run/docker.sock:/var/run/docker.sock")

BIND_DIR := "dist"
DOCKER_MOUNT := -v "$(CURDIR)/$(BIND_DIR):/go/src/github.com/jive/postal/$(BIND_DIR)"

DOCKER_RUN := docker run $(INTEGRATION_OPTS) --net=host -it $(DOCKER_ENVS) $(DOCKER_MOUNT) "$(BUILD_IMG)"

build: dist
	docker build -t "$(BUILD_IMG)" -f build.Dockerfile .

dist:
	mkdir dist

test:
	docker-compose up -d
	$(DOCKER_RUN) ./scripts/build.sh test-unit combine-coverage
	docker-compose down

dist/postal_%: build
	$(DOCKER_RUN) ./scripts/crossbinary $@

docker: dist/postal_linux-amd64
	./scripts/docker-build $(GIT_BRANCH)

clean:
	rm -rf dist
