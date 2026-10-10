FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -o /out/go-inventory .

FROM alpine:3.19
RUN adduser -D -u 10001 app
USER app
COPY --from=build /out/go-inventory /usr/local/bin/go-inventory
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/go-inventory"]
