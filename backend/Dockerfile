FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY cmd ./cmd
RUN CGO_ENABLED=0 go build -buildvcs=false -o /out/chess-server ./cmd/server

FROM alpine:3.20 AS runtime
ENV STOCKFISH_PATH=/opt/stockfish/stockfish-ubuntu-x86-64-avx2
RUN apk add --no-cache ca-certificates curl tar \
  && mkdir -p /opt/stockfish \
  && curl -L https://github.com/official-stockfish/Stockfish/releases/download/sf_18/stockfish-ubuntu-x86-64-avx2.tar -o /tmp/stockfish.tar \
  && echo "536c0c2c0cf06450df0bfb5e876ef0d3119950703a8f143627f990c7b5417964  /tmp/stockfish.tar" | sha256sum -c - \
  && tar -xf /tmp/stockfish.tar -C /opt/stockfish --strip-components=1 stockfish/stockfish-ubuntu-x86-64-avx2 \
  && chmod +x /opt/stockfish/stockfish-ubuntu-x86-64-avx2 \
  && rm /tmp/stockfish.tar \
  && adduser -D -H chess
COPY --from=build /out/chess-server /usr/local/bin/chess-server
USER chess
EXPOSE 3000
CMD ["chess-server"]
