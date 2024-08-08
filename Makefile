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
	npm install -g typescript
	npm install -g rollup
	npm i --save-dev @types/jquery
	npx tsc

.PHONY: release
release: install-packr install-typescript
	curl -sL https://git.io/goreleaser | bash

.PHONY: clean
clean:
	packr clean
	rm managementd
