# Immich Stacker

A small application to help you stack images in [Immich](https://immich.app).

The Immich web app allows you to manually stack your files but doesn't give you any
automation. This app adds a small automation layer that can be run periodically to
stack your photos.

## Use Cases

Below are some sample use cases.

### Raw + JPG

You shoot in Raw+JPG on your DSLR / smart phone and you'd like Immich to dedupe the
photos while preserving the raw files. The below config searches for `RW2` and `JPG`
files with the same filename other than the extension and stacks them, with the `JPG`
becoming the parent image.

```bash
IMMICH_MATCH=\.(JPG|RW2)$
IMMICH_PARENT=\.JPG$
```

### Burst Mode

Modern smart phones provide a burst option. Below is a config for detecting bursts of
photos and stacking them with the cover image becoming the parent.

```bash
IMMICH_MATCH=BURST[0-9]{3}(_COVER)?\.jpg$
IMMICH_PARENT=_COVER\.jpg$
```

## Versions

* 1.0.0 - works up to Immich v1.106.0
* 1.1.1 - works up to Immich v1.112.0
* 1.1.2 - works up to Immich v1.112.0
* 1.2.0 - works up to latest

## Deployment

Running the application is straightforward but it only runs once; there is no loop.
Configuration is taken from the environment or a `.env` file. Care should be taken
when using special characters in an environment variable. `.env` files will handle
escaping for you but a docker deployment needs care.

### Standalone

Download the prebuilt binary from Github and run it. The example below uses a `.env`
file for repeatability and to minimise mistakes when escaping regexes.

```bash
cat > .env << EOF
IMMICH_API_KEY=abc123
IMMICH_ENDPOINT=https://immich.app/api
IMMICH_MATCH=\.(JPG|RW2)$
IMMICH_PARENT=\.JPG$
EOF

./immich-stacker
```

### Docker

Use the prebuilt docker container. The example below uses a `.env` file for
repeatability and to minimise mistakes when escaping regexes.

```bash
cat > .env << EOF
IMMICH_API_KEY=abc123
IMMICH_ENDPOINT=https://immich.app/api
IMMICH_MATCH=\.(JPG|RW2)$
IMMICH_PARENT=\.JPG$
EOF

docker run -ti --rm --env-file=.env mattdavis90/immich-stacker-latest
```

### Using Swarm Cronjobs

Since the stacker only runs once then exits. It is recommended to use a cron scheduler
if deploying on Docker Swarm or in a docker-compose file. `crazymax/swarm-cronjob`
works reliably and is easy to configure.

**Note:** Take care when escaping regex in a docker-compose file.

```yaml
services:
  stacker:
    image: mattdavis90/immich-stacker:latest
    deploy:
      replicas: 0
      labels:
        - "swarm.cronjob.enable=true"
        - "swarm.cronjob.schedule=0 * * * *"
        - "swarm.cronjob.skip-running=false"
      restart_policy:
        condition: none
    environment:
      IMMICH_API_KEY: abc123
      IMMICH_ENDPOINT: "https://immich.com/api"
      IMMICH_MATCH: "\\.(JPG|RW2)$$"
      IMMICH_PARENT: "\\.JPG$$"
  swarm-cronjob:
    image: crazymax/swarm-cronjob
    deploy:
      placement:
        constraints:
          - node.role == manager
    environment:
      - "TZ=Europe/London"
      - "LOG_LEVEL=info"
      - "LOG_JSON=false"
    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock"
```

### Additional Options

There is an `IMMICH_LOG_LEVEL` environment variable that accepts a standard log level
and will configure the output of the applciation. It defaults to `INFO`.

The `IMMICH_COMPARE_CREATED` environment variable can be used to only match files with
the same createdAt timestamp. This can be useful when your camera has reset numbering to
0 and you can no longer rely on the filenames being unique. Thanks to @Pikachews for
the suggestion.

The `IMMICH_INSECURE_TLS` environment variable can be used to disable the checking of
TLS certificates. This option should be used with caution, for this reason it emit a
warning log on startup.
