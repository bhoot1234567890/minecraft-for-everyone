FROM gradle:8-jdk21 AS builder
WORKDIR /app
COPY . .
RUN gradle shadowJar --no-daemon

FROM alpine:latest
COPY --from=builder /app/build/libs/*.jar /OptionalAuthGuard.jar
USER 1000:1000
CMD ["sh", "-c", "mkdir -p /output && cp /OptionalAuthGuard.jar /output/"]
