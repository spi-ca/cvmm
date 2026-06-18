# syntax=docker/dockerfile:1

##
## Build
##
#FROM registry.example.invalid/library/golang:1.21.0-alpine AS build
FROM public.ecr.aws/docker/library/golang:1.21.1 AS build

WORKDIR /build

COPY internal internal/
COPY main.go go.mod go.sum ./

RUN set -xe && \
    go mod download && \
    go mod verify && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o cvmm .

##
## Deploy
##
#FROM registry.example.invalid/library/alpine:3.14
FROM public.ecr.aws/docker/library/debian:sid-slim
LABEL maintainer="tom.ato <maintainer@example.invalid>"
COPY --from=build /build/cvmm /usr/local/bin/cvmm

ARG UID=1111
ARG GID=1111

RUN set -xeu && \
    apt update && \
    DEBIAN_FRONTEND=noninteractive \
    apt install -y --no-install-recommends \
    curl \
    bash \
    iproute2 \
    aptitude \
    strace \
    lsof \
    tini \
    gnupg \
    sudo \
    htop \
    rclone \
    ca-certificates \
    dstat \
    git && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*


RUN set -xeu && \
    addgroup --gid $GID bc-user && \
    adduser \
    --disabled-login \
    --home "/home/bc-user" \
    --gecos "" \
    --shell /bin/bash \
    --gid $GID \
    --uid $UID \
    "bc-user" && \
    echo 'bc-user ALL=(root) NOPASSWD:ALL' > /etc/sudoers.d/bc-user && \
    chmod 0440 "/etc/sudoers.d/bc-user" && \
    chown -R bc-user:bc-user "/home/bc-user"

USER bc-user:bc-user
WORKDIR /home/bc-user
ENV PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
ENTRYPOINT [ "/usr/local/bin/cvmm" ]
