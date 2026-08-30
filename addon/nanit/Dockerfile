# hadolint ignore=DL3006
ARG BUILD_FROM

# --- build the nanit binary from source -------------------------------------
FROM golang:1.24-alpine AS build
RUN apk add --no-cache git ca-certificates
ARG NANIT_REF=main
RUN git clone --depth 1 --branch "${NANIT_REF}" \
      https://github.com/WColan/home_assistant_nanit /src \
 || git clone --depth 1 https://github.com/WColan/home_assistant_nanit /src \
      && git -C /src fetch --depth 1 origin "${NANIT_REF}" && git -C /src checkout FETCH_HEAD
WORKDIR /src
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /nanit ./cmd/nanit

# --- runtime --------------------------------------------------------------------
FROM ${BUILD_FROM}
RUN apk add --no-cache ffmpeg
COPY --from=build /nanit /usr/bin/nanit
COPY rootfs /
RUN chmod a+x /usr/bin/nanit /run.sh
CMD [ "/run.sh" ]
