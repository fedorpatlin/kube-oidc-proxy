# Copyright Jetstack Ltd. See LICENSE for details.
FROM alpine:3.10
LABEL description="OIDC reverse proxy authenticator based on Kubernetes"

RUN apk --no-cache add ca-certificates \
    && apk --no-cache add --upgrade openssl \
    && rm /bin/busybox

COPY ./bin/kube-oidc-proxy-linux /usr/bin/kube-oidc-proxy

ENTRYPOINT ["/usr/bin/kube-oidc-proxy"]

USER 999
