FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/forge ./cmd/forge

FROM alpine:3.19
RUN adduser -D -u 10001 forge
USER forge
COPY --from=build /out/forge /usr/local/bin/forge
EXPOSE 7000
ENTRYPOINT ["/usr/local/bin/forge"]
