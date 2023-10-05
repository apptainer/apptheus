ARG ARCH="amd64"
ARG OS="linux"
FROM quay.io/prometheus/busybox-${OS}-${ARCH}:latest
LABEL maintainer="The Apptheus Contributors"

ARG ARCH="amd64"
ARG OS="linux"
COPY --chown=nobody:nobody apptheus /bin/apptheus

EXPOSE 9091
RUN mkdir -p /apptheus && chown nobody:nobody /apptheus
WORKDIR /apptheus

USER 65534

ENTRYPOINT [ "/bin/apptheus" ]
