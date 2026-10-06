FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN go mod tidy
RUN go test ./...
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/yar-server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/yar-server /yar-server
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/yar-server"]
