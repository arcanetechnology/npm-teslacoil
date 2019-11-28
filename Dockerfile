FROM golang:1.13 AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
# We don't want to copy everything, but exclusions are handled by the .dockerignore file
COPY . .

# Build the Go app
RUN GOOS=linux make build

# Make teslacoil executable
RUN chmod a+x tlc 

WORKDIR /root

# Copy the binary
RUN cp /app/tlc .


# Copy the DB migrations
RUN cp -r /app/db/migrations migrations

# Copy the sample RSA key we'll use when working locally 
RUN cp /app/contrib/sample-private-pkcs1-rsa.pem .
