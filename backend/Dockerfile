# Build context is the repo root: this image needs the shared go.mod/go.sum
# in addition to backend/ itself.
#   docker build -f backend/Dockerfile .
FROM golang:1.25-alpine@sha256:56961d79ea8129efddcc0b8643fd8a5416b4e6228cfd477e3fd61deb2672c587 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY backend ./backend
RUN CGO_ENABLED=0 go build -o /out/server ./backend/cmd/server
RUN mkdir -p /out/data

# gcr.io/distroless/static-debian12:nonroot
FROM gcr.io/distroless/static-debian12@sha256:b7ebc675e6df3e26840de28a9d119969806b5542902111bfd111964d0930c08a
COPY --from=build /out/server /server
COPY --from=build --chown=65532:65532 /out/data /data
EXPOSE 8080 9000 9100
ENTRYPOINT ["/server"]
