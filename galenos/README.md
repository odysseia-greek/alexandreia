# Galenos

Galenos contains integration and system tests for the Alexandrian services. The
first suite exercises the public Dionysios gRPC contract with Ginkgo.

The baseline deliberately checks only externally useful guarantees:

- service health and loaded grammar mappings;
- representative grammar analysis and audit output;
- request validation for grammar, research, and text mode;
- a Herodotus reference passage that exercises grammar, canonical dictionary
  resolution, known-text discovery, and the evidence intended for Demosthenes.

## Run

Start Dionysios and its dependencies, then run from the repository root:

```sh
go test ./galenos/...
```

The suite uses `localhost:50060` by default. Override the target when testing a
container or cluster deployment:

```sh
DIONYSIOS_GRPC_ADDRESS=dionysios.example:50060 go test ./galenos/...
```

To use Ginkgo's reporting and filtering, install its CLI and run:

```sh
ginkgo -r ./galenos
```
