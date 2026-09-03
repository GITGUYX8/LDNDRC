# Build the control panel as a static binary so the runtime image needs no
# shell, compiler, or Go installation.
FROM golang:1.26.5-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags='-s -w' -o /out/ldndrc-control-panel ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/ldndrc-control-panel /ldndrc-control-panel
EXPOSE 8082
USER nonroot:nonroot
ENTRYPOINT ["/ldndrc-control-panel"]
