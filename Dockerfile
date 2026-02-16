FROM node:18.19.0 AS FRONT
WORKDIR /web
COPY ./web .
RUN yarn install --frozen-lockfile --network-timeout 1000000 && yarn run build

FROM golang:1.20.12 AS BACK
WORKDIR /go/src/caswaf
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o server . \
    && apt update && apt install wait-for-it && chmod +x /usr/bin/wait-for-it

FROM alpine:latest AS STANDARD
LABEL MAINTAINER="https://caswaf.org/"

COPY --from=BACK /go/src/caswaf/ ./
COPY --from=BACK /usr/bin/wait-for-it ./
RUN mkdir -p web/build && apk add --no-cache bash coreutils tzdata
COPY --from=FRONT /web/build /web/build
ENTRYPOINT ["./wait-for-it", "db:3306", "--", "./server"]

