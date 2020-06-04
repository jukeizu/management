FROM golang:1.14 as build
WORKDIR /go/src/github.com/jukeizu/management
COPY Makefile go.mod go.sum ./
RUN make deps
ADD . .
RUN make build-linux
RUN echo "nobody:x:100:101:/" > passwd

FROM scratch
COPY --from=build /go/src/github.com/jukeizu/management/passwd /etc/passwd
COPY --from=build --chown=100:101 /go/src/github.com/jukeizu/management/bin/management .
USER nobody
ENTRYPOINT ["./management"]
