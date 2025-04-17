# bakah

Build [Bake] files using [Buildah].

## Usage

bakah can be used to build JSON Bake files:

    bakah docker-bake.json

bakah can also be used to build Docker Compose projects:

    docker compose build --print 2>&1 | bakah -f -

## License

MIT

[Bake]: https://docs.docker.com/build/bake/introduction/
[Buildah]: https://buildah.io/
