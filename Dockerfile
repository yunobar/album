FROM golang:1.26-alpine AS build-stage

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY internal/adapters/db ./internal/adapters/db

COPY . .

RUN go install github.com/swaggo/swag/cmd/swag@latest && \
    ./scripts/swag-init.sh

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -buildvcs=false \
    -ldflags='-w -s' \
    -o /http ./cmd/http

FROM gcr.io/distroless/static-debian12 AS build-release-stage

WORKDIR /

COPY --from=build-stage /http /http

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/http"]
