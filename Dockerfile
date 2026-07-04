FROM golang:1.23-bookworm AS build

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /out/ai-tool-analytics ./cmd/ai-tool-analytics

FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/ai-tool-analytics /app/ai-tool-analytics
VOLUME ["/data"]
ENV CUA_ADDR=:4318
ENV CUA_DB=/data/ai-tool-analytics.sqlite
EXPOSE 4318

ENTRYPOINT ["/app/ai-tool-analytics"]
