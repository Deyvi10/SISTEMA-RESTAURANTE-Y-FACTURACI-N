# Imagen de las herramientas de desarrollo (simulador de impresoras y stub del SRI).
# Uso: docker build -f deploy/docker/tools.Dockerfile --build-arg TOOL=printer-sim .
FROM golang:1.27-alpine AS build
ARG TOOL
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY packages ./packages
COPY tools ./tools
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./tools/${TOOL}

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot
ENTRYPOINT ["/app"]
