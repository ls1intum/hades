FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY ./HadesWebhook/go.mod ./HadesWebhook/go.sum ./HadesWebhook/
RUN cd HadesWebhook && go mod download

COPY ./HadesWebhook ./HadesWebhook

WORKDIR /app/HadesWebhook
RUN CGO_ENABLED=0 go build -o hades-webhook .

FROM gcr.io/distroless/static-debian12

WORKDIR /app

COPY --from=builder /app/HadesWebhook/hades-webhook .

CMD ["/app/hades-webhook"]
