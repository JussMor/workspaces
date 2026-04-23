FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/forge ./cmd/forge

FROM alpine:3.19
RUN adduser -D -u 10001 forge
WORKDIR /home/forge
RUN mkdir -p /home/forge/data && chown -R forge:forge /home/forge
USER forge
COPY --from=build /out/forge /usr/local/bin/forge
EXPOSE 7000
ENTRYPOINT ["/usr/local/bin/forge"]
