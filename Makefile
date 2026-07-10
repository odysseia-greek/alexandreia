PROTO_DIRS := aristarchos dionysios eratosthenes

.PHONY: docs-grpc tidy tidy-go mods


.PHONY: tools
tools: $(PROTOC_GEN_DOC)

$(PROTOC_GEN_DOC):
	@mkdir -p $(TOOLS_DIR)
	@echo "Installing protoc-gen-doc..."
	@GOBIN=$(TOOLS_DIR) go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@latest


.PHONY: docs-grpc
docs-grpc: tools
	@for dir in $(PROTO_DIRS); do \
		echo "Generating gRPC docs in $$dir..."; \
		PATH=$(TOOLS_DIR):$$PATH buf generate --template $$dir/buf.gen.docs.yaml $$dir; \
	done

.PHONY: tidy tidy-go
tidy: tidy-go

tidy-go:
	@echo "==> Running 'go mod tidy' in all modules..."
	@mods=$$(find . -name go.mod -print0 | xargs -0 -n1 dirname | sort -u); \
	if [[ -z "$$mods" ]]; then \
		echo "No go.mod files found."; \
		exit 0; \
	fi; \
	for d in $$mods; do \
		echo "==> go mod tidy in $$d"; \
		( cd "$$d" && go mod tidy && go fmt ./... ); \
	done

# Convenience: list all module dirs found
.PHONY: mods
mods:
	@find . -name go.mod -print0 | xargs -0 -n1 dirname | sort -u
