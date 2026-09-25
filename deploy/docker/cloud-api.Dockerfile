# Imagen de la API de la nube (F1-14). Binario estático sin CGO sobre distroless (sin shell).
# docker build -f deploy/docker/cloud-api.Dockerfile -t restpos/cloud-api .
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# nodynamic: el codificador WebP usa su versión WebAssembly embebida en vez de cargar libwebp
# del sistema; así el binario es estático y corre en distroless.
RUN CGO_ENABLED=0 go build -tags nodynamic -trimpath -ldflags="-s -w" -o /out/cloud-api ./apps/cloud-api/cmd/cloud-api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/cloud-api /cloud-api
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/cloud-api"]
CMD ["serve"]
