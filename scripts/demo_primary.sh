#!/bin/bash

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

echo "Primary REGISTRY running at http://localhost:5001"
echo "Primary BOOSTER  running at http://localhost:5002"

docker logs -f $PRIMARY_BOOSTER_ID
