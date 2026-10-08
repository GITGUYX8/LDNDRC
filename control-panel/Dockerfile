# Student SPA (R4 UI-1+): built here so the image always ships the real UI,
# even though web/dist/ stays gitignored. Falls back to the placeholder when
# web/package.json is absent (e.g. partial contexts).
FROM node:22-bookworm-slim AS webbuild
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm install --no-audit --no-fund 2>/dev/null || npm install --no-audit --no-fund || true
COPY web/ ./
RUN if [ -f package.json ]; then npm run build; else mkdir -p dist && echo "no web build"; fi

# Build the control panel as a static binary so the runtime image needs no
# shell, compiler, or Go installation.
FROM golang:1.26.5-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Overlay the freshly built SPA (dist/ is gitignored, so COPY above only
# carries the placeholder/.gitkeep on fresh clones).
COPY --from=webbuild /web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags='-s -w' -o /out/ldndrc-control-panel ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/ldndrc-control-panel /ldndrc-control-panel
EXPOSE 8082
USER nonroot:nonroot
ENTRYPOINT ["/ldndrc-control-panel"]
