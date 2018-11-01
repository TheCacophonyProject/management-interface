.PHONY: build-arm
build-arm:
	GOARCH=arm GOARM=7 packr build -ldflags="-s -w" ./cmd/managementd

.PHONY: build
build:
	packr build -ldflags="-s -w" ./cmd/managementd

.PHONY: clean
clean:
	packr clean
	rm managementd
