Freeplay Mode
-------------

The "freeplay" involves the server sending the client a sequence of
board states (`state`) that the client can respond to (`move`). The
server MAY restrict the time a client has to respond (`stop`), that
the client MAY also give up by their own accord (`yield`). IDs and
references SHOULD be used to ensure the correct and unambitious
association between requests and answers.

A server might use the `freeplay` mode to implement a tournament, as
seen in this example:

	s: kgp 1 0 0
	c: mode freeplay
	s: 4 state <3,0,0,3,3,3,3,3,3>
	c: @4 move 1
	s: 6@4 stop
	s: 8 state <3,1,3,0,4,4,4,3,3>
	c: @8 move 3
	c: @8 move 2
	c: @8 yield
	s: 10@8 stop
	...

Where `s:` are commands sent out by the server, and `c:` by the
client.

There are no requirements on how a server is to send out
`state`-requests and on how long the client is given to respond.

Freeplay commands
=================

The following commands must be understood for a client to implement
the "freeplay" mode:
  
`state [board]` (server)

: Sends the client a board state to work on. The command SHOULD have
  an ID so that later `move`, `yield` and `stop` commands can safely
  reference the request they are responding to, without interfering
  with other concurrent requests.
  
  The client always interprets the request as making the move as the
  "south" player.

`move [integer]` (client)

: In response to a `state` command, the client informs the server of
  their preliminary decision. Multiple `move` commands can be sent
  out, iteratively improving over the previous decision.
  
  An integer $n$ designates the $n$'th pit, that is to say uses
  1-based numbering.  The value must be in between $1 \leq n < s$,
  where $s$ is the board size.

`stop` (server)

: An indication by the server that it is not interested in any more
  `move` commands for some `state` request. Any `move` command sent
  out after a `stop` MUST be silently ignored.
  
  If the client has not sent a `move` command, the server MUST make a
  random decision for the client.

`yield` (client)

: The voluntary indication by a client that the last `move` command
  was the best it could decide, and that it will not be responding to
  the referenced `state` command any more. The client sending a
  `yield` command is analogous to a server sending `stop`.
