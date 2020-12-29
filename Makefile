BEAGLE := "debian@beaglebone"

VERSION := $(shell git describe --always --dirty)

RELEASE_DIR := wossa-$(VERSION)
RELEASE_FILE := wossa-$(VERSION).tar.gz

build:
	cd cmd && go build -o ../wossa

build-beagle:
	cd cmd && env GOOS=linux GOARCH=arm GOARM=7 go build -o ../wossa -v -ldflags="-s -w -X main.version=$(VERSION)"

release: build-beagle
	rm -rf release || true
	mkdir -p release/$(RELEASE_DIR)
	cp -r wossa wossa.service public release/$(RELEASE_DIR)/
	cd release && tar czf $(RELEASE_FILE) $(RELEASE_DIR)

install: release
	scp release/$(RELEASE_FILE) install-wossa.sh $(BEAGLE):
	ssh -t $(BEAGLE) sudo bash install-wossa.sh $(RELEASE_FILE)

clean:
	rm -f wossa
	rm -rf release
