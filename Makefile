.PHONY: generate tailwind build run test vet tidy

# Regenerate Go code from .templ files.
generate:
	go run github.com/a-h/templ/cmd/templ@v0.3.1020 generate

# Build the Tailwind CSS bundle. Uses `npx` (bundled with Node.js) rather
# than requiring the standalone tailwindcss CLI to be installed separately;
# pinned to v3 since our config/input.css use the v3 syntax.
tailwind:
	npx --yes tailwindcss@3 -i web/tailwind/input.css -o web/static/css/app.css -c web/tailwind/tailwind.config.js --minify

build: generate tailwind
	go build -o bin/server ./cmd/server

run: generate tailwind
	go run ./cmd/server

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy
