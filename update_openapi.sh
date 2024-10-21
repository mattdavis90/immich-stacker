#!/bin/bash

VERSION=1.118.2

pushd client
wget https://raw.githubusercontent.com/immich-app/immich/v${VERSION}/open-api/immich-openapi-specs.json -O immich-openapi-specs.json
popd
go generate ./...
