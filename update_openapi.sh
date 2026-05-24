#!/bin/sh

VERSION=2.7.5

pushd client
curl https://raw.githubusercontent.com/immich-app/immich/v${VERSION}/open-api/immich-openapi-specs.json -o immich-openapi-specs.json
popd
go generate ./...
