FROM golang:1.17 AS build
ARG BUILD

WORKDIR /go/src/github.com/stellarproject/heimdall
COPY . /go/src/github.com/stellarproject/heimdall
RUN make

FROM alpine:latest
COPY --from=build /go/src/github.com/stellarproject/heimdall/bin/* /bin/
ENTRYPOINT ["/bin/heimdall"]
