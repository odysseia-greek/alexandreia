# Makefile for version release management

# Default module to release
MODULE ?= archytas

# Get the latest version tag for the specified module
.PHONY: get-latest-version
get-latest-version:
	@echo "Getting latest version for $(MODULE)..."
	@latest_tag=$$(git tag -l "$(MODULE)/v*" | sort -V | tail -n 1); \
	if [ -z "$$latest_tag" ]; then \
		echo "No existing tags found for $(MODULE), using v0.1.0 as base"; \
		echo "$(MODULE)/v0.1.0"; \
	else \
		echo "Latest version: $$latest_tag"; \
	fi

# Release a new patch version (e.g., v0.1.2 -> v0.1.3)
.PHONY: release-patch
release-patch:
	@echo "Releasing new patch version for $(MODULE)..."
	@latest_tag=$$(git tag -l "$(MODULE)/v*" | sort -V | tail -n 1); \
	if [ -z "$$latest_tag" ]; then \
		new_tag="$(MODULE)/v0.1.0"; \
		echo "No existing tags found for $(MODULE), using $$new_tag as first version"; \
	else \
		version=$$(echo $$latest_tag | sed 's/$(MODULE)\/v//'); \
		major=$$(echo $$version | cut -d. -f1); \
		minor=$$(echo $$version | cut -d. -f2); \
		patch=$$(echo $$version | cut -d. -f3); \
		new_patch=$$((patch + 1)); \
		new_tag="$(MODULE)/v$$major.$$minor.$$new_patch"; \
		echo "Incrementing patch version: $$latest_tag -> $$new_tag"; \
	fi; \
	git tag $$new_tag && git push origin $$new_tag; \
	echo "Released: $$new_tag"

# Release a new minor version (e.g., v0.1.2 -> v0.2.0)
.PHONY: release-minor
release-minor:
	@echo "Releasing new minor version for $(MODULE)..."
	@latest_tag=$$(git tag -l "$(MODULE)/v*" | sort -V | tail -n 1); \
	if [ -z "$$latest_tag" ]; then \
		new_tag="$(MODULE)/v0.1.0"; \
		echo "No existing tags found for $(MODULE), using $$new_tag as first version"; \
	else \
		version=$$(echo $$latest_tag | sed 's/$(MODULE)\/v//'); \
		major=$$(echo $$version | cut -d. -f1); \
		minor=$$(echo $$version | cut -d. -f2); \
		new_minor=$$((minor + 1)); \
		new_tag="$(MODULE)/v$$major.$$new_minor.0"; \
		echo "Incrementing minor version: $$latest_tag -> $$new_tag"; \
	fi; \
	git tag $$new_tag && git push origin $$new_tag; \
	echo "Released: $$new_tag"

# Release a new major version (e.g., v0.1.2 -> v1.0.0)
.PHONY: release-major
release-major:
	@echo "Releasing new major version for $(MODULE)..."
	@latest_tag=$$(git tag -l "$(MODULE)/v*" | sort -V | tail -n 1); \
	if [ -z "$$latest_tag" ]; then \
		new_tag="$(MODULE)/v1.0.0"; \
		echo "No existing tags found for $(MODULE), using $$new_tag as first version"; \
	else \
		version=$$(echo $$latest_tag | sed 's/$(MODULE)\/v//'); \
		major=$$(echo $$version | cut -d. -f1); \
		new_major=$$((major + 1)); \
		new_tag="$(MODULE)/v$$new_major.0.0"; \
		echo "Incrementing major version: $$latest_tag -> $$new_tag"; \
	fi; \
	git tag $$new_tag && git push origin $$new_tag; \
	echo "Released: $$new_tag"

# Help command
.PHONY: help
help:
	@echo "Usage:"
	@echo "  make get-latest-version [MODULE=module_name]  - Get the latest version for a module"
	@echo "  make release-patch [MODULE=module_name]       - Release a new patch version (x.y.z -> x.y.z+1)"
	@echo "  make release-minor [MODULE=module_name]       - Release a new minor version (x.y.z -> x.y+1.0)"
	@echo "  make release-major [MODULE=module_name]       - Release a new major version (x.y.z -> x+1.0.0)"
	@echo ""
	@echo "Examples:"
	@echo "  make release-patch MODULE=archytas            - Release a new patch version for archytas"
	@echo "  make release-minor MODULE=plato               - Release a new minor version for plato"
	@echo ""
	@echo "If MODULE is not specified, 'archytas' is used as the default."