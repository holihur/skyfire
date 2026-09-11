BINARY   := skyfired
CLIENT   := skyfire-client
PREFIX   ?= /usr/local
ETC      ?= /etc/skyfire
BINROOT  := $(PREFIX)/bin
EMBDIR   := backend/internal/web/dist
BINDIR   := backend
CLIENTDIR := client

.PHONY: all web backend client client-windows client-darwin install uninstall run demo test clean

all: web backend

# Build the React frontend and stage it into the Go embed directory.
web:
	cd frontend && pnpm install && pnpm build
	mkdir -p $(EMBDIR)
	cp -r frontend/dist/index.html frontend/dist/assets $(EMBDIR)/

backend:
	cd $(BINDIR) && go build -trimpath -ldflags "-s -w" -o ../$(BINARY) ./cmd/skyfired

# Desktop client for the current platform (tray on Windows; terminal
# fallback on Linux / macOS without cgo).
client:
	cd $(CLIENTDIR) && go build -trimpath -ldflags "-s -w" -o ../$(CLIENT) .

# Windows client with system tray (pure Go, no cgo needed).
client-windows:
	cd $(CLIENTDIR) && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o ../$(CLIENT)-windows-amd64.exe .

# macOS client with system tray. The tray needs cgo (Cocoa), so run this on
# macOS. `CGO_ENABLED=0 GOOS=darwin go build` also works anywhere and yields
# the terminal fallback.
client-darwin:
	cd $(CLIENTDIR) && GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o ../$(CLIENT)-darwin-arm64 .

install: all
	install -d $(DESTDIR)$(BINROOT) $(DESTDIR)$(ETC)
	install -m 0755 $(BINARY) $(DESTDIR)$(BINROOT)/skyfired
	install -m 0644 -D deploy/skyfire.service $(DESTDIR)/lib/systemd/system/skyfire.service
	echo "Installed. Password is generated at first start and printed to the journal:"
	echo "  journalctl -u skyfire -n 20"

# run the daemon with the embedded UI (needs root for real tuning; use `make demo`)
run: all
	sudo ./$(BINARY) -config $(ETC)/config.json -addr :51821

# safe development mode: mock driver, sample data, no system changes.
demo: all
	./$(BINARY) -demo -addr :51821

test:
	cd $(BINDIR) && go vet ./... && go test ./...
	cd $(CLIENTDIR) && go vet ./... && go build ./...

clean:
	rm -f $(BINARY) $(CLIENT) $(CLIENT)-windows-amd64.exe $(CLIENT)-darwin-arm64
	rm -rf frontend/dist
	rm -rf $(EMBDIR)
