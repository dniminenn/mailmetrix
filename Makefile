export CGO_ENABLED=0

all: plugins mailmetrix

# Build the main application
mailmetrix:
	go build -o bin/mailmetrix cmd/main.go

# optional plugins
plugins:
	@if [ -f plugins.mk ]; then $(MAKE) -f plugins.mk; else echo "No plugins.mk found. Skipping plugins."; fi

clean:
	@if [ -f plugins.mk ]; then $(MAKE) -f plugins.mk clean; fi
	rm -f bin/*

.PHONY: all mailmetrix plugins clean
