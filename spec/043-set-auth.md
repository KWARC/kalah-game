`auth`-group
------------

In cases where an identity has to be preserved over multiple
connections (a tournament or other competitions), some kind of
authentication is required. The `auth` group consists of a single
variable to implement this as simply as possible:

`auth:token` (string)

: As soon as the client sends sets this option, the server will
  associate the current client with any previous client that has used
  the same token. No registration is necessary, and the server MAY
  decide to abort the connection if the token is not secure enough.

  The value of the token must be a non-empty string.

The client SHOULD use an encrypted connection when using the auth
group, as to avoid MITM attacks.  The server MUST NOT reject
connections that do not set `auth:token`.
