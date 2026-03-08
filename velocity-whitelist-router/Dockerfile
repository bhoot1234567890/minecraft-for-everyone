# Build stage
FROM gradle:8-jdk21 AS builder
WORKDIR /app
COPY . .
RUN gradle shadowJar --no-daemon

# Output stage - just copy the JAR to output
FROM alpine:latest
COPY --from=builder /app/build/libs/*.jar /WhitelistRouter.jar
CMD ["sh", "-c", "cp /WhitelistRouter.jar /output/"]
