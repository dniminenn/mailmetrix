# plugins.example.mk
# Example Makefile for managing webmail plugins
# Copy this file to `plugins.mk` and modify it as needed.

# Example plugin URL (fictional)
EXAMPLE_PLUGIN_URL=https://example.com/plugins/example-plugin.go
EXAMPLE_PLUGIN_PATH=webmailtester/example-plugin.go

# Default rule to fetch all plugins
all: fetch-example-plugin

# Rule to fetch the example plugin
fetch-example-plugin:
	@echo "Fetching example-plugin.go..."
	@if [ ! -d "webmailtester" ]; then mkdir -p webmailtester; fi
	@curl -sSL $(EXAMPLE_PLUGIN_URL) -o $(EXAMPLE_PLUGIN_PATH) || (echo "Failed to fetch example-plugin.go" && exit 1)

# Clean up downloaded plugins
clean:
	@echo "Cleaning up plugins..."
	rm -f $(EXAMPLE_PLUGIN_PATH)

.PHONY: all fetch-example-plugin clean
