FROM golang:latest as build

WORKDIR /build
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /app/app ./cmd

FROM bitnami/minideb:latest
WORKDIR /app
COPY --from=build /app/app /bin/app
RUN apt-get update && apt-get install -y wget gnupg2 lsb-release
RUN sh -c 'echo "deb https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list' && \
    wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add - && \
    apt-get update

CMD ["app"]
