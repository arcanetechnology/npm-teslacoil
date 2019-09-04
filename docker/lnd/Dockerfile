# This Dockerfile is based on the official LND Dockerfile by roasbeef

FROM golang:1.12-alpine as builder

# The version of LND to install
ARG LND_VERSION

# Force Go to use the cgo based DNS resolver. This is required to ensure DNS
# queries required to connect to linked containers succeed.
ENV GODEBUG netdns=cgo

# Install dependencies and install/build lnd.
RUN apk add --no-cache --update alpine-sdk \
    git \
    make \
&&  mkdir -p $GOPATH/src/github.com/lightningnetwork/lnd \
&&  cd $GOPATH/src/github.com/lightningnetwork/lnd \
&&  git clone https://github.com/lightningnetwork/lnd.git . \ 
&&  git checkout "v$LND_VERSION" \
&&  make \
&&  make install tags="signrpc walletrpc chainrpc invoicesrpc routerrpc"

# Start a new, final image to reduce size.
FROM alpine as final

# Add bash.
RUN apk add --no-cache \
    bash

# Copy the entrypoint script.
COPY "start-lnd.sh" .
RUN chmod a+x start-lnd.sh

# Prepare logging dir for LND
RUN mkdir -p /data
RUN chmod a+rw /data

# user ID
ARG UID

# group ID
ARG GID

# Create new user
ARG UNAME=lnd
RUN addgroup -g $GID $UNAME
RUN adduser -u $UID -G $UNAME -D -s /bin/bash $UNAME

# Run as newly created user
USER $UNAME

# Expose lnd ports (server, rpc).
EXPOSE 9735 10009

# Copy the binaries and entrypoint from the builder image.
COPY --from=builder /go/bin/lncli /bin/
COPY --from=builder /go/bin/lnd /bin/

