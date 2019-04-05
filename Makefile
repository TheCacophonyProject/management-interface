.PHONY: build-arm
build-arm: install-packr
	GOARCH=arm GOARM=7 packr build -ldflags="-s -w" ./cmd/managementd

.PHONY: install-packr
install-packr:
	go get github.com/gobuffalo/packr/packr 

.PHONY: build
build: install-packr
	packr build -ldflags="-s -w" ./cmd/managementd

.PHONY: release	
release: install-packr
	curl -sL https://git.io/goreleaser | bash

.PHONY: clean
clean:
	packr clean
	rm managementd