# Build context is the repo root: this image needs the shared go.mod/go.sum
# in addition to backend/ itself.
#   docker build -f backend/Dockerfile .
FROM golang:1.27.4-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY backend ./backend
RUN CGO_ENABLED=0 go build -o /out/server ./backend/cmd/server
RUN mkdir -p /out/data

# gcr.io/distroless/static-debian12:nonroot
FROM gcr.io/distroless/static-debian12@sha256:d75cdd72874d4790092fcb1b058493ecf6bb5bf2b2b897045b00ff01d91843f2
COPY --from=build /out/server /server
COPY --from=build --chown=65532:65532 /out/data /data
EXPOSE 8080 9001 9100
ENTRYPOINT ["/server"]
