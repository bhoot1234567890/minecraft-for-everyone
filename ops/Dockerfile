FROM alpine:3.20

RUN apk add --no-cache bash coreutils curl findutils netcat-openbsd tzdata

COPY state-backup-loop.sh /ops/state-backup-loop.sh
COPY stack-monitor.sh /ops/stack-monitor.sh

RUN chmod +x /ops/state-backup-loop.sh /ops/stack-monitor.sh
