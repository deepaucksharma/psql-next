FROM alpine:3.18

RUN apk --no-cache add ca-certificates

WORKDIR /

COPY database-intelligence /database-intelligence
COPY production-config-complete.yaml /etc/database-intelligence/config.yaml

EXPOSE 4317 4318 13133

ENTRYPOINT ["/database-intelligence"]
CMD ["--config", "/etc/database-intelligence/config.yaml"]