FROM golang:1.15-alpine as build

WORKDIR /src
ADD . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w"
RUN apk add upx
RUN upx metadata

FROM scratch
COPY --from=build /src/metadata /bin/metadata
ENTRYPOINT [ "/bin/metadata" ]
