BEAGLE := "debian@beaglebone"

VERSION := $(shell git describe --always --dirty)

RELEASE_DIR := wossamessa-$(VERSION)
RELEASE_FILE := wossamessa-$(VERSION).tar.gz

build:
	cd cmd && go build -o ../wossamessa

build-beagle:
	cd cmd && env GOOS=linux GOARCH=arm GOARM=7 go build -o ../wossamessa -v -ldflags="-s -w -X main.version=$(VERSION)"

release: build-beagle
	rm -rf release || true
	mkdir -p release/$(RELEASE_DIR)
	cp -r wossamessa wossamessa.service public release/$(RELEASE_DIR)/
	cd release && tar czf $(RELEASE_FILE) $(RELEASE_DIR)

install: release
	scp release/$(RELEASE_FILE) install-wossamessa.sh $(BEAGLE):
	ssh -t $(BEAGLE) sudo bash install-wossamessa.sh $(RELEASE_FILE)

clean:
	rm -f wossamessa
	rm -rf release
