FROM golang:1.26-alpine AS build
WORKDIR /src
COPY protocol /protocol
COPY metrology .
RUN go build -o /metrology .
FROM alpine:3.21
COPY --from=build /metrology /usr/local/bin/metrology
USER 0
ENTRYPOINT ["metrology"]
