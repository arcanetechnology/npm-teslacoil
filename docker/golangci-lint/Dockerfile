# Cribbed from https://gitlab.com/sorenmat/golangci-lint/blob/master/Dockerfile

FROM golang:1.12-alpine

RUN wget https://github.com/golangci/golangci-lint/releases/download/v1.7.2/golangci-lint-1.7.2-linux-amd64.tar.gz
RUN tar -zxvf golangci-lint-1.7.2-linux-amd64.tar.gz
RUN mv golangci-lint-1.7.2-linux-amd64/golangci-lint /bin