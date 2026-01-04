# Base build
FROM golang:1.25-alpine as base

ARG project_name
ARG TARGETOS
ARG TARGETARCH

ENV project_name=${project_name}
ENV TARGETOS=${TARGETOS}
ENV TARGETARCH=${TARGETARCH}

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Debug build
FROM base as debug
# Install Air for live reloading
RUN go install github.com/air-verse/air@latest
# Install Delve for debugging
RUN go install github.com/go-delve/delve/cmd/dlv@latest
EXPOSE 2345
CMD ["air", "-c", ".air.toml"]

# Build binary with Go
FROM base as builder
ENV CGO_ENABLED=0

ARG project_name
ENV project_name=${project_name}

RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /app/${project_name}

# Production build
FROM alpine:3.23.2 as prod
WORKDIR /app

ARG project_name
ENV project_name=${project_name}

COPY --from=builder /app/${project_name} .
ENV TMPDIR=/tmp
ENTRYPOINT [ "sh", "-c", "/app/${project_name}" ]
