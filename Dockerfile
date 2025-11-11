FROM cgr.dev/chainguard/go:latest AS build

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -a -buildvcs=false -trimpath -ldflags="-s -w -buildid=" -o /app/app .

FROM bitnami/minideb:latest

WORKDIR /app
RUN install_packages wget lsb-release ca-certificates && \
    wget --quiet -O /etc/apt/trusted.gpg.d/postgresql.asc https://www.postgresql.org/media/keys/ACCC4CF8.asc && \
    echo "deb https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list && \
    apt remove -y wget lsb-release && apt autoremove -y && apt update && install_packages postgresql-client-18

COPY --from=build /app/app /usr/local/bin/postgres-backup

CMD ["postgres-backup", "backup"]
