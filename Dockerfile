FROM golang:1.13.1 as bd
RUN adduser --disabled-login --gecos "" appuser
WORKDIR /github.com/layer5io/meshery-cpx
ADD . .
RUN GOPROXY=direct GOSUMDB=off go build -ldflags="-w -s" -a -o /meshery-cpx .
RUN find . -name "*.go" -type f -delete; mv cpx /
RUN wget -O /istio-1.3.0.tar.gz https://github.com/istio/istio/releases/download/1.3.0/istio-1.3.0-linux.tar.gz
RUN wget -O /citrix-istio-adaptor-1.1.0-beta.tar.gz https://github.com/citrix/citrix-istio-adaptor/archive/v1.1.0-beta.tar.gz

FROM alpine
# Install kubectl
ADD https://storage.googleapis.com/kubernetes-release/release/v1.16.0/bin/linux/amd64/kubectl /usr/local/bin/kubectl
RUN apk --update add  openssl && \
    rm -rf /var/cache/apk/*
RUN set -x && \
    apk --update add --no-cache curl ca-certificates && \
    chmod +x /usr/local/bin/kubectl && \
    kubectl version --client

RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
COPY --from=bd /meshery-cpx /app/
COPY --from=bd /cpx /app/cpx
COPY --from=bd /etc/passwd /etc/passwd
ENV ISTIO_VERSION=1.3.0
ENV CIA_VERSION=v1.1.0-beta
USER appuser
WORKDIR /app
CMD ./meshery-cpx
