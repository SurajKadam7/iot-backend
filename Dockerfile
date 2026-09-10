FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /src/web/dist /app/web/dist
ENV WEB_DIST_DIR=/app/web/dist
EXPOSE 8080
USER 65532:65532
ENTRYPOINT ["/app/server"]
