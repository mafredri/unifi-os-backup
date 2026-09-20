FROM golang:1.27.1-trixie AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/unifi-os-backup ./cmd/unifi-os-backup

FROM gcr.io/distroless/static-debian13:nonroot
WORKDIR /backups
COPY --from=build /out/unifi-os-backup /usr/local/bin/unifi-os-backup
VOLUME ["/backups"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s CMD ["/usr/local/bin/unifi-os-backup", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/unifi-os-backup"]
