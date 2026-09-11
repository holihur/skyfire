BINARY   := skyfired
PREFIX   ?= /usr/local
ETC      ?= /etc/skyfire
BINROOT  := $(PREFIX)/bin
EMBDIR   := backend/internal/web/dist
BINDIR   := backend

.PHONY: all web backend install uninstall run demo test clean

all: web backend

# Build the React frontend and stage it into the Go embed directory.
web:
	cd frontend && pnpm install && pnpm build
	mkdir -p $(EMBDIR)
	cp -r frontend/dist/index.html frontend/dist/assets $(EMBDIR)/

backend:
	cd $(BINDIR) && go build -trimpath -ldflags "-s -w" -o ../$(BINARY) ./cmd/skyfired

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

clean:
	rm -f $(BINARY)
	rm -rf frontend/dist
	rm -rf $(EMBDIR)