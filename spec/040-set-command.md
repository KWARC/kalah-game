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

On connecting, the server may inform the client about itself. The
options of this group are:

`info:server-name` (string)

: A name of the server or the implementation

`info:online` (integer)

: The number of currently connected clients.

`time`-group
------------

For "freeplay" and especially "simple", the server may indicate how it
manages the time a client is given. The options of this group are:

`time:system` (word)

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

Reserved for future versions of the specification.

