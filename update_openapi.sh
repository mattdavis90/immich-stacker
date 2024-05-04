#!/bin/bash

pushd client
wget https://raw.githubusercontent.com/immich-app/immich/main/open-api/immich-openapi-specs.json -O immich-openapi-specs.json
popd
go generate ./...
