FROM golang:1-alpine AS compiled
RUN apk --no-cache add git

WORKDIR /go/src/github.com/jonstaryuk/logwatch
COPY . .
RUN go test -timeout 30s ./...
RUN go install github.com/jonstaryuk/logwatch/cmd
RUN git rev-parse HEAD | tr -d '\n' > /commit.sha

FROM alpine
WORKDIR /app
COPY --from=compiled /commit.sha /commit.sha
COPY --from=compiled /go/bin/cmd .
ENTRYPOINT ["./cmd"]
