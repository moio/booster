# 🚀 booster

Makes synchronization of container images between registries faster.

## Requirements

 - Two or more container registries to synchronize
 - Access to their backing storage directory (for now)

## Demo

Start two registry instances backed by local directories:
```shell
docker run -d \
  -p 5001:5001 \
  -v `pwd`/primary:/var/lib/registry \
  registry:2

docker run -d \
  -p 5002:5002 \
  -v `pwd`/replica:/var/lib/registry \
  registry:2
```

Start a companion booster instance for each:

```shell
git clone git@github.com:moio/booster.git
cd booster
go run main.go serve --port 5011 ../primary &
go run main.go serve --port 5012 --primary=http://localhost:5001 ../replica &
```

Synchronize replica with primary:
```shell
curl http://localhost:5012/sync
```
