# Giant Bomb Video Grabber

This simple Go application will use your [Giant Bomb API key](https://www.giantbomb.com/api/) to download all the latest [Giant Bomb](https://www.giantbomb.com) videos to watch offline at your leisure. Useful if you have a home media server and want to set up a cron job that grabs all the latest videos on a schedule, or just want an easy way to get a local copy of everything.

## Installation

Get the latest version for your OS from the [releases page](https://github.com/benmurden/GB-Video-Grabber/releases) and extract wherever you like.

## Usage

Upon first running the app it will generate a default config file called `config.yaml`. Edit this to set your API key and you're all ready to go.

## Config options

### `apikey`
Set this to your [Giant Bomb API key](https://www.giantbomb.com/api/). Requires a [Giant Bomb](https://www.giantbomb.com) account.

### `videodir`
Set this to the desired directory for videos.
Default: `./videos/`

### `maxconcurrency`
Maximum number of concurrent downloads.
Default: `3`

## Command line options

Any of the above options can be overridden at runtime using an invocation flag of the same name.

E.g. `--maxconcurrency=1` will change the concurrency so that downloads come in one at a time. May help reduce network load and disk thrashing.

## Motivation

Mostly I wanted to learn about Go and some of its concurrency paradigms. This kind of app also requires learning a few other aspects of the language in order to cover a few requirements:

- Making web requests
- JSON API consumption
- Parsing responses
- Queueing jobs
- Showing progress in the terminal
- Config handling
- Date parsing
- Writing files to disk
- Resuming interrupted downloads

## License
MIT