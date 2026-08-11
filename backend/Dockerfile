# Build context is the repo root: this image needs the shared go.mod/go.sum
# in addition to backend/ itself.
#   docker build -f backend/Dockerfile .
# golang:1.25-alpine
FROM golang@sha256:56961d79ea8129efddcc0b8643fd8a5416b4e6228cfd477e3fd61deb2672c587 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY backend ./backend
RUN CGO_ENABLED=0 go build -o /out/server ./backend/cmd/server

# gcr.io/distroless/static-debian12:latest
FROM gcr.io/distroless/static-debian12@sha256:6447365a6337c3732f412d1b74357b30a633831955b2bc45552b0086be907687
COPY --from=build /out/server /server
EXPOSE 8080 9000 9100
ENTRYPOINT ["/server"]
