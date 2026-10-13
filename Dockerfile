# syntax=docker/dockerfile:1
# OpenPak's Unity Gaming Services backend. Plain HTTP inside the pod network —
# Traefik terminates the guest's TLS for the Unity hostnames, the same arrangement
# as servers/t17 and servers/klei.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/ugsd ./cmd/ugsd

FROM gcr.io/distroless/static
COPY --from=build /out/ugsd /ugsd
USER nonroot:nonroot
EXPOSE 22680 22681
ENTRYPOINT ["/ugsd"]
