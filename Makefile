.PHONY: build-arm
build-arm: install-packr
	GOARCH=arm GOARM=7 packr build -ldflags="-s -w" ./cmd/managementd

.PHONY: install-packr
install-packr:
	go install github.com/gobuffalo/packr/packr@v1.30.1

.PHONY: build
build: install-packr install-typescript
	packr build -ldflags="-s -w" ./cmd/managementd


.PHONY: install-typescript
install-typescript:
	npm install typescript
	npm install rollup
	npx tsc

.PHONY: release
release: install-packr
	curl -sL https://git.io/goreleaser | bash

.PHONY: clean
clean:
	packr clean
	rm managementd
