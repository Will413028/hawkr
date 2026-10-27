FROM golang:1.25.7-alpine@sha256:f6751d823c26342f9506c03797d2527668d095b0a15f1862cddb4d927a7a4ced AS builder
WORKDIR /app
COPY main.go .
RUN CGO_ENABLED=0 go build -o /discord-webhook-gateway main.go

FROM alpine:3.22@sha256:14358309a308569c32bdc37e2e0e9694be33a9d99e68afb0f5ff33cc1f695dce
COPY --from=builder /discord-webhook-gateway /discord-webhook-gateway
CMD ["/discord-webhook-gateway"]
