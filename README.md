# forest-go

[![builds.sr.ht status](https://builds.sr.ht/~whereswaldon/forest-go.svg)](https://builds.sr.ht/~whereswaldon/forest-go?)
[![GoDoc](https://godoc.org/git.sr.ht/~whereswaldon/forest-go?status.svg)](https://godoc.org/git.sr.ht/~whereswaldon/forest-go)

This repo contains:

- A golang library for working with nodes in the Arbor Forest. This repo is based on the work-in-progress specification [available here](https://github.com/arborchat/protocol/blob/forest/spec/Forest.md).
- A CLI for creating, manipulating, and viewing nodes in the Arbor Forest in `cmd/forest`

For information about each component of this repo, see later in this file.

## About Arbor

![arbor logo](https://git.sr.ht/~whereswaldon/forest-go/blob/master/img/arbor-logo.png)

Arbor is a chat system that makes communication clearer. It explicitly captures context that other platforms ignore, allowing you to understand the relationship between each message and every other message. It also respects its users and focuses on group collaboration.

You can get information about the Arbor project [here](https://man.sr.ht/~whereswaldon/arborchat/).

For news about the project, join our [mailing list](https://lists.sr.ht/~whereswaldon/arbor-dev)!

### Using the CLI

Right now, the CLI works with files in its current working directory, though this will change in the future.
For the meantime, create a directory to play around in:

```sh
mkdir arbor-forest
cd arbor-forest
```

> **Important: About OpenPGP Keys**
> 
> Arbor Forest nodes are signed by OpenPGP private keys. This gives Arbor strong guarantees about the authenticity of messages. The below procedures assume that you have `gpg2` installed and have already [generated a private key](https://fedoraproject.org/wiki/Creating_GPG_Keys#Creating_GPG_Keys_Using_the_Command_Line). Wherever you see `--gpguser <email>` below, substitute the email address associated with your GPG private key for `<email>`.
> 
> If you do not have `gpg2` or a key and you do not want to install them, you can omit the `--gpguser <email>` flag in the commands below. If you do this, the CLI will create a new one for you and store it
> in `./arbor.privkey`. This private key is not encrypted (has no passphrase), and should not be used for anything of importance.

#### Identities

Since all nodes must be signed by an Identity node, you must create one of those before you can create any others.

```sh
forest create identity --name <your-name> --gpguser <email>
```

This will print the base64url-encoded ID of your identity node, which will be stored in a file by that name in your
current working directory.


To view your identity in a human-readable format, try the following (install `jq` if you don't have it, it's really handy):

```sh
forest show <id> | jq .
```

Substitute the base64url-encoded ID of your identity node for `<id>`. `jq` will pretty-print the JSON to make it easier to read.

#### Communities

To create a community, use:

```sh
forest create community --as <id> --name <community-name> --gpguser <email>
```

Substitute the base64url-encoded ID of your identity node for `<id>` and provide appropriate values for name and metadata.

To view your community in a human-readable format, try the following:

```sh
forest show <id> | jq .
```

Substitute the base64url-encoded ID of your community node for `<id>`. `jq` will pretty-print the JSON to make it easier to read.

#### Replies

To create a reply, use:

```sh
forest create reply --as <id> --to <parent-id> --content <your message> --gpguser <email>
```

Substitute the base64url-encoded ID of your identity node for `<id>` and the base64url-encoded ID of another reply or conversation node for `<parent-id>`. Substitute `<your message>`
for the content of your reply. Usually this will be a response to the content of the node referenced by `<parent-id>`.

To view your reply in a human-readable format, try the following:

```sh
forest show <id> | jq .
```

Substitute the base64url-encoded ID of your reply node for `<id>`. `jq` will pretty-print the JSON to make it easier to read.


## Build

Must use Go 1.11+

`go build`

## Test

`go test -v -cover`
