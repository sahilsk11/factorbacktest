# Minimal spike image — NOT the production Factor API binary.
# Records boot time so cold-start measurements can separate container start from app init.
FROM alpine:3.20
RUN apk add --no-cache curl
ENV BOOT_ISO=
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
EXPOSE 8080
CMD ["/entrypoint.sh"]
