# syntax=docker/dockerfile:1

# ---- Stage 1: Tailwind CSS ------------------------------------------------
# Scans internal/templates/**/*.templ for utility classes and produces the
# final app.css that gets embedded into the Go binary in stage 2.
FROM node:22-bookworm-slim AS tailwind
WORKDIR /src
COPY web/tailwind ./web/tailwind
COPY internal/templates ./internal/templates
RUN npx --yes tailwindcss@3 \
    -i web/tailwind/input.css \
    -o web/static/css/app.css \
    -c web/tailwind/tailwind.config.js \
    --minify

# ---- Stage 2: Go build -----------------------------------------------------
FROM golang:1.27-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Bring in the CSS built in stage 1 before go:embed picks up web/static/.
COPY --from=tailwind /src/web/static/css/app.css ./web/static/css/app.css

RUN go run github.com/a-h/templ/cmd/templ@v0.3.1020 generate

# CGO disabled: modernc.org/sqlite is a pure-Go SQLite driver, so a fully
# static binary is possible — required for the distroless "static" base
# image below.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# ---- Stage 3: runtime -------------------------------------------------------
# distroless static + nonroot: no shell, no package manager, runs as an
# unprivileged user by default — about as small an attack surface as a
# container image gets.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app
COPY --from=build /out/server /app/server

ENV PORT=8080
ENV DATA_DIR=/data
VOLUME ["/data"]
EXPOSE 8080

ENTRYPOINT ["/app/server"]
