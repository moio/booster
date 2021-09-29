# 🚀 booster

Booster is a research project to improve distribution of container images, focusing especially on air-gapped and Edge use cases.

The PoC in this repository allows fast transfer of container images between registries, saving 20% to 80% bandwidth compared to `docker pull`/`docker push`.

[![golangci-lint](https://github.com/moio/booster/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/moio/booster/actions/workflows/golangci-lint.yml)


## Requirements

 - Two or more container registries to synchronize
 - Access to their backing storage directory (for now)

## Demo

Set up a demo environment with two local Registry containers, each with its booster:
```shell

# Start a "primary" registry backed by a local directory
docker run -d \
  -p 5001:5000 \
  -v `pwd`/primary:/var/lib/registry \
  registry:2

# Start a companion booster container for the primary
PRIMARY_BOOSTER_ID=$( \
docker run -d \
  -p 5002:5000 \
  -v `pwd`/primary:/var/lib/registry \
  ghcr.io/moio/booster:latest \
) 


# Start a "replica" registry backed by another local directory
docker run -d \
  -p 5003:5000 \
  -v `pwd`/replica:/var/lib/registry \
  registry:2

# Start a companion booster container for the replica
# Link it to the primary booster
docker run -d \
  -p 5004:5000 \
  -v `pwd`/replica:/var/lib/registry \
  --link $PRIMARY_BOOSTER_ID:primary-booster \
  ghcr.io/moio/booster:latest --primary=http://primary-booster:5000
```

Load up the primary Registry with an image:
```shell
docker pull ubuntu:bionic-20210615.1
docker image tag ubuntu:bionic-20210615.1 localhost:5001/ubuntu:bionic-20210615.1
docker image push localhost:5001/ubuntu:bionic-20210615.1
```

Synchronize the replica to the primary's contents via:
```shell
curl http://localhost:5004/sync
```

Then push another image and synchronize again:
```shell
docker pull ubuntu:bionic-20210702
docker image tag ubuntu:bionic-20210702 localhost:5001/ubuntu:bionic-20210702
docker image push localhost:5001/ubuntu:bionic-20210702

curl http://localhost:5004/sync
```

To clean up temporary files:
```shell
curl http://localhost:5002/cleanup
curl http://localhost:5004/cleanup
```

Scripts to set up and tear down a demo with two replicas are available in the `scripts` directory.

## Hacking

Building of release binaries, packages and Docker images is done via [goreleaser](https://goreleaser.com).

For a snapshot build use:

```shell
goreleaser release --skip-publish --snapshot --rm-dist
```
