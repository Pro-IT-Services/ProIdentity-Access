.PHONY: build build-ui run dev clean

UI_DIR  = webui
DIST    = internal/api/ui/dist
SERVER  = bin/proidentity

build: build-ui
	go build -o $(SERVER) ./cmd/server/

build-ui:
	cd $(UI_DIR) && npm ci && npm run build
	rm -rf $(DIST)
	cp -r $(UI_DIR)/dist $(DIST)

run: build
	./$(SERVER) config.yaml

dev-server:
	go run ./cmd/server/ config.yaml

dev-ui:
	cd $(UI_DIR) && npm run dev

clean:
	rm -rf $(SERVER) $(DIST) $(UI_DIR)/dist

install-server:
	install -m 755 $(SERVER) /usr/local/bin/proidentity
	install -m 640 config.yaml /etc/proidentity/config.yaml

.PHONY: migrate
migrate:
	mariadb -u wgmanager -pwgmanager_dev_pass wgmanager < migrations/001_initial.sql
