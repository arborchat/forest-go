/*
Package forest is a library for creating nodes in the Arbor Forest data structure.

The specification for the Arbor Forest can be found here: https://github.com/arborchat/protocol/blob/forest/spec/Forest.md

NOTE: this package requires using a fork of golang.org/x/crypto, and you must therefore include the following in your go.mod:

    replace golang.org/x/crypto => github.com/ProtonMail/crypto <version-from-forest-go's-go.mod>

All nodes in the Arbor Forest are cryptographically signed by an Identity
node. Identity nodes sign themselves. To create a new identity, first create or load an OpenPGP private key using golang.org/x/crypto/openpgp. Then you can use that key and name to create an identity.

    privkey := getPrivateKey() // do this however
    name, err := fields.NewQualifiedContent(fields.ContentTypeUTF8, "example")
    // handle error
    metadata, err := fields.NewQualifiedContent(fields.ContentTypeJSON, "{}")
    // handle error
    identity, err := forest.NewIdentity(privkey, name, metadata)
    // handle error

Identities (and their private keys) can be used to create other nodes with
the Builder type. You can create community nodes using a builder like so:

    builder := forest.As(identity, privkey)
    communityName, err := fields.NewQualifiedContent(fields.ContentTypeUTF8, "example")
    // handle error
    communityMetadata, err := fields.NewQualifiedContent(fields.ContentTypeJSON, "{}")
    // handle error
    community, err := builder.NewCommunity(communityName, communityMetadata)
    // handle error

Builders can also create reply nodes:

    message, err := fields.NewQualifiedContent(fields.ContentTypeUTF8, "example")
    // handle error
    replyMetadata, err := fields.NewQualifiedContent(fields.ContentTypeJSON, "{}")
    // handle error
    reply, err := builder.NewReply(community, message, replyMetadata)
    // handle error
    message2, err := fields.NewQualifiedContent(fields.ContentTypeUTF8, "reply to reply")
    // handle error
    reply2, err := builder.NewReply(reply, message2, replyMetadata)
    // handle error

The Builder type can also be used fluently like so:

    // omitting creating the qualified content and error handling
    community, err := forest.As(identity, privkey).NewCommunity(communityName, communityMetadata)
    reply, err := forest.As(identity, privkey).NewReply(community, message, replyMetadata)
    reply2, err := forest.As(identity, privkey).NewReply(reply, message2, replyMetadata)

*/
package forest
