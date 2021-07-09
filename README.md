# 🚀 booster

Makes synchronization of container images between registries faster.

![build status](https://github.com/moio/booster/actions/workflows/checks.yml/badge.svg)

## Requirements

 - Two or more container registries to synchronize
 - Access to their backing storage directory (for now)

## Demo

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


Now replica can be synchronized to the primary's contents via:
```shell
curl http://localhost:5004/sync
```



## Hacking

Building of release binaries, packages and Docker images is done via [goreleaser](https://goreleaser.com).

For a snapshot build use:

```shell
goreleaser release --skip-publish --snapshot --rm-dist
```