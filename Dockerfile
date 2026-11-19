FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /notifier ./cmd/notifier && mkdir /data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /notifier /notifier
# Same UID as the old Python image, so the existing notifier-data volume stays writable.
COPY --from=build --chown=65532:65532 /data /data
USER 65532:65532
WORKDIR /data
EXPOSE 8080
ENTRYPOINT ["/notifier"]
