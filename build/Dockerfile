# syntax=docker/dockerfile:1
# One Dockerfile for every service:  docker build --build-arg APP=dex-api -f build/Dockerfile .
#
# Base images are pinned by digest (Erika's lesson): a tag can be re-pointed,
# a digest can't. Renovate/Dependabot bump these for you.
FROM golang:1.27-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG APP
RUN test -n "$APP" && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app "./cmd/${APP}"

# distroless/static: no shell, no package manager, runs as UID 65532.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /out/app /app
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app"]
