FROM golang:1.26.0-alpine AS build
WORKDIR /src/gateway
COPY protocol /src/protocol
COPY gateway /src/gateway
RUN go build -o /app .
FROM alpine:3.22
COPY --from=build /app /app
ENV DATA_DIR=/data
EXPOSE 8082
ENTRYPOINT ["/app"]
