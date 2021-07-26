#!/bin/bash

docker ps | grep ghcr.io/moio/booster:latest | awk '{print $1}' | xargs docker kill >/dev/null
docker ps -a | grep ghcr.io/moio/booster:latest | awk '{print $1}' | xargs docker rm >/dev/null

docker ps | grep registry:2 | awk '{print $1}' | xargs docker kill >/dev/null
docker ps -a | grep registry:2 | awk '{print $1}' | xargs docker rm >/dev/null
