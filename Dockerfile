FROM cgr.dev/chainguard/go:latest as build

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod tidy

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app/app ./cmd

FROM bitnami/minideb:latest

WORKDIR /app
RUN install_packages wget gnupg2 lsb-release ca-certificates && \
    echo "deb https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list && \
    wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add - && \
    apt remove -y wget gnupg2 lsb-release && apt autoremove -y && apt update

COPY --from=build /app/app /bin/postgres-backup

CMD ["postgres-backup", "upload-schedule"]
