FROM golang:1.26.2-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
RUN go build -buildvcs=false -trimpath -o /out/api ./cmd/api && \
    go build -buildvcs=false -trimpath -o /out/worker ./cmd/worker

FROM alpine:3.23
WORKDIR /app
COPY --from=build /out/api /out/worker ./
USER 65532:65532
ENV HTTP_ADDR=0.0.0.0:8080
EXPOSE 8080
ENTRYPOINT ["/app/api"]
