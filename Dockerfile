FROM golang:1.13.1 as bd
RUN adduser --disabled-login --gecos "" appuser
WORKDIR /github.com/layer5io/meshery-cpx
ADD . .
RUN GOPROXY=direct GOSUMDB=off go build -ldflags="-w -s" -a -o /meshery-cpx .
RUN find . -name "*.go" -type f -delete; mv cpx /

FROM alpine
RUN apk --update add ca-certificates
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
COPY --from=bd /meshery-cpx /app/
COPY --from=bd /cpx /app/cpx
COPY --from=bd /etc/passwd /etc/passwd
ENV CPX_VERSION=1.0
USER appuser
WORKDIR /app
CMD ./meshery-cpx
