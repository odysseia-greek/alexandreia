# Galenos

Galenos is the Ginkgo integration-test suite for the Dionysios gRPC API. Its
suite is kept at the module root so it can be compiled into one test binary,
matching the deployment model used by Dareios.

## Configure the endpoint

Set `DIONYSIOS_GRPC_ADDRESS` to the Dionysios gRPC address. It defaults to
`localhost:50060`.

```shell
export DIONYSIOS_GRPC_ADDRESS=localhost:50060
```

## Run locally

```shell
go test -v ./...
```

Or with the Ginkgo CLI:

```shell
ginkgo -v
```

## Build and run the container

```shell
docker build -f Containerfile -t galenos .
docker run --rm \
  -e DIONYSIOS_GRPC_ADDRESS=host.docker.internal:50060 \
  galenos
```

The suite exercises the public Dionysios contract and checks:

- Service health and loaded grammar mappings.
- Representative grammar analysis and audit output.
- Request validation for grammar, research, and text mode.
- A Herodotus reference passage that exercises grammar, canonical dictionary
  resolution, known-text discovery, and the evidence intended for Demosthenes.
