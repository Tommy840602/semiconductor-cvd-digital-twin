FROM golang:1.26.0-alpine AS build
WORKDIR /src/bridge
COPY protocol /src/protocol
COPY bridge /src/bridge
RUN go build -o /app .
FROM alpine:3.22
COPY --from=build /app /app
ENV DATA_DIR=/data
EXPOSE 8083
ENTRYPOINT ["/app"]
