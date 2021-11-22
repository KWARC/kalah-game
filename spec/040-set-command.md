The `set` Command
=================

The `set` command may be used at any time by both client and server to
inform the other side about capabilities, internal states, rules,
etc. The structure of a set command is

	set [option] [value]

Each option is structured using colons (`:`) to group commands
together. Each command group specified here SHOULD be implemented
entirely by both client and server:

`info`-group
------------

On connecting, server and client may inform each other about each other. The
options of this group are:

`info:name` (string)

: The codename of the client or the server.

`info:authors` (string)

: Authors who wrote the client

`info:description` (string)

: A brief description of the client's algorithm.

`info:comment` (string)

: Comment of the client about the current game state and it's chosen
  move.  Might contain (depending on the algorithm), number of nodes,
  search depth, evaluation, ...

`time`-group
------------

For "freeplay" and especially "simple", the server may indicate how it
manages the time a client is given. The options of this group are:

`time:mode` (word)

: One of `none` when no time is tracked, `absolute` if the client is
  given an absolute amount of time it may use and `relative` if the
  time used by a client for one `state` request has no effect on the
  time that may be used for other requests.
  
`time:clock` (integer)

: Number of seconds a client has left. This option MAY be set by the
  server before issuing a `state` command.
  
`time:opclock` (integer)

: Number of seconds an opponent has left.

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
