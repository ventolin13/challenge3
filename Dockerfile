FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /out/challenge3 ./cmd/challenge3

FROM debian:bookworm-slim
COPY --from=build /out/challenge3 /challenge3
EXPOSE 3000
ENTRYPOINT ["/challenge3"]
