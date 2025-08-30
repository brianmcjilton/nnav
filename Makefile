APP=nnav

.PHONY: build run clean rpm deb pack snapshot

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o $(APP) .

run: build
	./$(APP)

clean:
	rm -f $(APP)
	rm -rf dist/

snapshot:
	goreleaser release --clean --snapshot

rpm deb: snapshot
	@echo "Packages will be in ./dist"

