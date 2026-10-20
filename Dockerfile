FROM golang:1.26.2-alpine AS build
WORKDIR /src
COPY . .
# This stage is shared by all services. Keep downloads before the service ARG
# and cache successful modules across builds when the network drops a transfer.
RUN --mount=type=cache,target=/go/pkg/mod \
    for attempt in 1 2 3; do \
      if go mod download; then exit 0; fi; \
      sleep 2; \
    done; exit 1
ARG SERVICE
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    if [ "$SERVICE" = "migrate" ]; then CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/migrate; else CGO_ENABLED=0 go build -trimpath -o /out/server ./services/${SERVICE}/cmd/${SERVICE}; fi

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /src/services /app/services
ENTRYPOINT ["/app/server"]
