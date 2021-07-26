#!/bin/bash

# Start a "replica" registry backed by a local directory
docker run -d \
  -p 5005:5000 \
  -v `pwd`/replica_2:/var/lib/registry \
  registry:2

# Start a companion booster container for the replica
# Link it to the primary booster
PRIMARY_BOOSTER_ID=$(docker ps | grep ghcr.io/moio/booster | grep -v primary | awk '{print $1}')
REPLICA_BOOSTER_ID=$( \
docker run -d \
  -p 5006:5000 \
  -v `pwd`/replica_2:/var/lib/registry \
  --link $PRIMARY_BOOSTER_ID:primary-booster \
  ghcr.io/moio/booster:latest --primary=http://primary-booster:5000
)

echo "Replica REGISTRY running at http://localhost:5005"
echo "Replica BOOSTER  running at http://localhost:5006"

docker logs -f $REPLICA_BOOSTER_ID
