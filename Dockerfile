# This is just a test Dockerfile made to create a container for testing.
FROM alpine:latest

RUN apk add --no-cache curl

CMD ["curl", "--version"]
