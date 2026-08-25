# Build context is the repo root: this image needs the shared go.mod/go.sum
# in addition to backend/ itself.
#   docker build -f backend/Dockerfile .
FROM golang:1.27-alpine@sha256:4c9fe60190a2a3350ddc51de80d0224b8a6698d12bdfc999fee45ea9d6c46dbc AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY backend ./backend
RUN CGO_ENABLED=0 go build -o /out/server ./backend/cmd/server
RUN mkdir -p /out/data

# gcr.io/distroless/static-debian12:nonroot
FROM gcr.io/distroless/static-debian12@sha256:6447365a6337c3732f412d1b74357b30a633831955b2bc45552b0086be907687
COPY --from=build /out/server /server
COPY --from=build --chown=65532:65532 /out/data /data
EXPOSE 8080 9000 9100
ENTRYPOINT ["/server"]
