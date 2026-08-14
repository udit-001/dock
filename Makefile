# Dock — common tasks.
# Note: Fyne/GLFW needs the unversioned X11 dev symlink (libXxf86vm.so) which
# this machine lacks; `deps` creates a local one and sets CGO_LDFLAGS.

APP      := app-store
BIN     := bin
OUT     := $(BIN)/$(APP)
LOCALLIB := $(HOME)/.local/lib
export CGO_LDFLAGS = -L$(LOCALLIB)

# Opt into Fyne's fyne.Do threading model so the driver's thread-safety
# warnings don't print. All our goroutine UI updates already go through
# fyne.Do, so this is safe and silences the "not been migrated" notice.
TAGS     := migrated_fynedo

VERSION_MAJOR := 0
VERSION_MINOR := 0
VERSION_PATCH := 1
GIT_COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)

LDFLAGS := -s -w

.PHONY: all deps build run test vet staticcheck check tidy gen install winres version inspect screenshot clean

# Pinned staticcheck (v0.7.0 / 2026.1) — go run keeps it out of go.mod.
STATICCHECK_VERSION := v0.7.0

all: build

# Local dev symlink for the missing unversioned Xlib dev lib.
deps:
	@mkdir -p $(LOCALLIB)
	@ln -sf /lib64/libXxf86vm.so.1 $(LOCALLIB)/libXxf86vm.so
	@go get gopkg.in/yaml.v3@latest fyne.io/fyne/v2@latest

build: deps
	@mkdir -p $(BIN)
	go build -trimpath -tags $(TAGS) -ldflags "$(LDFLAGS)" -o $(OUT) ./cmd/$(APP)

# View the desktop app. (Off-screen captures aren't shown to the user; run this
# on a machine with a display.)
run: build
	$(OUT)

screenshot:
	@echo "View the app with 'make run' on a display. Off-screen capture isn't supported."

test:
	go test ./...

vet:
	go vet ./...

staticcheck:
	go run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./...

# Static analysis gate: vet + staticcheck (tests are separate — see `test`).
check: vet staticcheck

# Dump the widget-tree markup (Playwright-style DOM snapshot) of the UI.
inspect: build
	./bin/$(APP) -inspect -

tidy:
	go mod tidy

# Regenerate the repo-root apps.json (jsDelivr source). The embedded copy is
# gone — the app fetches the snapshot at runtime and caches it locally.
gen:
	go run ./cmd/manifest-gen -manifest internal/appdata/manifest.yaml -out apps.json

# Attach Windows PE resource metadata (company/version/DpiAware) via go-winres.
winres:
	./scripts/gen-winres.sh $(VERSION_MAJOR).$(VERSION_MINOR).$(VERSION_PATCH)

install:
	go install -trimpath -tags $(TAGS) -ldflags "$(LDFLAGS)" ./cmd/$(APP)

version:
	@echo "v$(VERSION_MAJOR).$(VERSION_MINOR).$(VERSION_PATCH) ($(GIT_COMMIT))"

clean:
	rm -rf $(BIN)