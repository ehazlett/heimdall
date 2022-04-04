FROM golang:1.17 AS build
ARG BUILD

WORKDIR /go/src/github.com/ehazlett/heimdall
COPY . /go/src/github.com/ehazlett/heimdall
RUN make

FROM alpine:latest
RUN apk add -U --no-cache redis wireguard-tools
COPY --from=build /go/src/github.com/ehazlett/heimdall/bin/* /bin/
CMD ["heimdall", "-h"]
