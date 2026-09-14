#!/bin/bash
# Bound to loopback only, matching the Quadlet unit: with TRUST_PROXY=true
# (the default), a client that could reach the container directly could
# spoof X-Forwarded-For and bypass the login rate limiter. Put a reverse
# proxy in front of this on the same host to actually expose it.
podman run --rm \
  --name points \
  --volume ./data:/data \
  --publish 127.0.0.1:8080:8080 \
  --env-file ./.env \
  quay.io/wkulhanek/points-tracker:latest
