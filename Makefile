TARGET_HOST := "pi@raspberrypi"

VERSION := $(shell git describe --always --dirty)

RELEASE_NAME := wossa-$(VERSION)-$(GOARCH)$(GOARM)
RELEASE_FILE := $(RELEASE_NAME).tar.gz

build:
	cd cmd && go build -o ../wossa

build-release:
	cd cmd && go build -o ../wossa -v -ldflags="-s -w -X main.version=$(VERSION)"

release: build-release
	rm -rf release/$(RELEASE_NAME)
	mkdir -p release/$(RELEASE_NAME)
	cp -r install.sh wossa wossa.service public release/$(RELEASE_NAME)/
	cd release && tar czf $(RELEASE_FILE) $(RELEASE_NAME)

install: release
	scp -r release/$(RELEASE_NAME) $(TARGET_HOST):
	ssh -t $(TARGET_HOST) sudo $(RELEASE_NAME)/install.sh

clean:
	rm -f wossa
	rm -rf release
