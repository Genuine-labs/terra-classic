# syntax=docker/dockerfile:1

## Build Image
FROM golang:1.20 as go-builder

ARG BUILDPLATFORM

# Install minimum necessary dependencies, build Cosmos SDK, remove packages
RUN apt update
RUN apt install -y curl git build-essential
# debug: for live editting in the image
RUN apt install -y vim


WORKDIR /core
COPY . /core

RUN LEDGER_ENABLED=false make build

RUN if [ ${BUILDPLATFORM} = "linux/amd64" ]; then \
        WASMVM_URL="libwasmvm.x86_64.so"; \
    elif [ ${BUILDPLATFORM} = "linux/arm64" ]; then \
        WASMVM_URL="libwasmvm.aarch64.so"; \     
    else \
        echo "Unsupported Build Platfrom ${BUILDPLATFORM}"; \
        exit 1; \
    fi; \
    cp /go/pkg/mod/github.com/classic-terra/wasmvm@v*/internal/api/${WASMVM_URL} /lib/${WASMVM_URL}

RUN make build-e2e-init

## Deploy image
FROM ubuntu:23.04

COPY --from=go-builder /core/build/initialization /bin/initialization
COPY --from=go-builder /lib/${WASMVM_URL} /lib/${WASMVM_URL}

# Docker ARGs are not expanded in ENTRYPOINT in the exec mode. At the same time,
# it is impossible to add CMD arguments when running a container in the shell mode.
# As a workaround, we create the entrypoint.sh script to bypass these issues.
RUN echo "#!/bin/bash\ninitialization \"\$@\"" >> entrypoint.sh && chmod +x entrypoint.sh

ENV HOME /core
WORKDIR $HOME

ENTRYPOINT ["./entrypoint.sh"]
