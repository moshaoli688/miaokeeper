# builder

FROM golang:1.17-alpine AS builder

WORKDIR /root/
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o miaokeeper .

# image

FROM alpine:latest
LABEL maintainer=BBAliance

# disabling cgo when built, so no need to install libc6-compat
RUN apk update \
        && apk upgrade \
        && apk add --no-cache \
        && apk add tzdata \
        ca-certificates \
        && update-ca-certificates 2>/dev/null || true

ENV TZ Asia/Shanghai

WORKDIR /miaokeeper/
COPY --from=builder /root/miaokeeper /miaokeeper/miaokeeper
VOLUME /miaokeeper/

EXPOSE 9876

ENTRYPOINT ["/miaokeeper/miaokeeper"]
