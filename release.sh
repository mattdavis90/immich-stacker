#!/bin/bash

version=$(git describe --tags)

podman build . -t ghcr.io/mattdavis90/immich-stacker:${version}
podman build . -t ghcr.io/mattdavis90/immich-stacker:latest
podman build . -t mattdavis90/immich-stacker:${version}
podman build . -t mattdavis90/immich-stacker:latest

podman push ghcr.io/mattdavis90/immich-stacker:${version}
podman push ghcr.io/mattdavis90/immich-stacker:latest
podman push mattdavis90/immich-stacker:${version}
podman push mattdavis90/immich-stacker:latest
