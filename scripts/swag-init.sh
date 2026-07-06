#!/bin/sh
swag init \
    --generalInfo main.go \
    --dir cmd/http,internal/adapters/http/handler,internal/domain/dto,internal/domain/entity \
    --output docs \
    --outputTypes go,json,yaml \
    --parseDependency \
    --parseInternal
