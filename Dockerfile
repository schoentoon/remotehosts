FROM golang:latest AS builder

WORKDIR /root

RUN git clone https://github.com/coredns/coredns

WORKDIR /root/coredns

ENV GOPROXY direct

RUN sed -i 's/forward\:forward/remotehosts\:github.com\/schoentoon\/remotehosts\nforward\:forward/' plugin.cfg

RUN make

FROM gcr.io/distroless/base

COPY --from=builder /root/coredns/coredns /bin/coredns

ENTRYPOINT [ "/bin/coredns" ]