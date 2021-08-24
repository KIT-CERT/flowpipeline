# Flow Pipeline

<!-- quick links here maybe -->

## About The Project

[bwNET](https://bwnet.belwue.de/) is a research project of the German federal
state of Baden-Württemberg which aims to provide innovative services within the
state's research and education network [BelWü](https://www.belwue.de). This
GitHub Org contains the code pertaining to the monitoring aspect of this
project.

This repo contains our flow processing toolkit which enables us and our users
to define pipelines for [goflow2](https://github.com/netsampler/goflow2)-compatible
flow messages. The flowpipeline project integrates most other parts of our flow
processing stack into a single piece of software which can be configured to
serve any function:

* accepting raw Netflow (using [goflow2](https://github.com/netsampler/goflow2))
* enriching the resulting flow messages ([examples/enricher](https://github.com/bwNetFlow/flowpipeline/tree/master/examples/enricher))
* writing to and reading from Kafka ([examples/localkafka](https://github.com/bwNetFlow/flowpipeline/tree/master/examples/localkafka))
* dumping flows to cli (e.g. [flowdump](https://github.com/bwNetFlow/flowpipeline/tree/master/examples/flowdump))
* providing metrics and insights ([examples/prometheus](https://github.com/bwNetFlow/flowpipeline/tree/master/examples/prometheus))

## Getting Started

To get going, choose one of the following deployment methods.

### Compile from Source
Clone this repo and use `go build .` to build the binary yourself.

By default, the binary will look for a `config.yml` in its local directory, so
you'll either want to create one or call it from any example directory (and
maybe follow the instructions there).

### Binary Releases
Download our [latest release](https://github.com/bwNetFlow/flowpipeline/releases)
and run it, same as if you compiled it yourself.

### Container Releases
A ready to use container is provided as `bwnetflow/flowpipeline`, you can check
it out on [docker hub](https://hub.docker.com/r/bwnetflow/flowpipeline).

Configurations referencing other files (geolocation databases for instance)
will work in a container without extra edits. This is because the volume
mountpoint `/config` is prepended in all segments which accept configuration to
open files, if the binary was built with the `container` build flag.

```sh
podman run -v ./examples/xy:/config flowpipeline
# or
docker run -v ./examples/xy:/config flowpipeline
```

## Usage

TODO

## Contributing

Contributions in any form (code, issues, feature requests) are very much welcome.
