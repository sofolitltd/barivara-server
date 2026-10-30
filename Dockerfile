FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -u 1001 appuser

WORKDIR /app
COPY barivara-server .
USER appuser
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/health >/dev/null || exit 1
CMD ["./barivara-server"]
