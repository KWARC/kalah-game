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
