FROM golang:1.26.0-alpine AS build
WORKDIR /src/simulator
COPY protocol /src/protocol
COPY simulator /src/simulator
RUN go build -o /app .
FROM alpine:3.22
COPY --from=build /app /app
ENV DATA_DIR=/data
EXPOSE 5000 8081
ENTRYPOINT ["/app"]
