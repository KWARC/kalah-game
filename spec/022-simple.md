Simple Mode
===========

The "simple" mode restricts "freeplay" by introducing a more strict
state model, thus relieving both client and server from having to
track IDs. The main intention is to make client implementations
easier, by shifting the burden of synchronisation management on to the
server.

As such "simple" mode is convenient for implementing tournaments
agents.

Simple commands
---------------

`state [board] / stop` (server)

: The server MUST alternate `state` and `stop` commands (possibly,
  with other commands inbetween of course), starting with a state
  command.

`yield` (client)

: The client MUST send `yield` after it has finished searching, no
  matter the reason. The client MUST not send `yield` in any other
  situation.

  There are three cases:

  * The server sends `stop`, the client replies with `yield`
  * The client sends `yield`, the server replies with `stop`
  * The client sends `yield` "at the same time" as the server sends
    `stop`, neither replies to the other (they already did)

Examples
--------

Example of a slow client (unrealistically short game):

	s: kgp 1 0 0
	c: mode simple
	s: state <4,0,0,3,0,0,0,1,1,1,1>
	c: move 1
	s: stop
	s: state <3,1,0,0,4,4,3,3,3>
	c: move 1
	c: move 1
	c: yield
	c: move 2
	c: move 4
	c: move 3
	s: stop
	c: mode 4
	c: yield
	s: goodbye

In this example the client did not realize the search period was ended
and keeps sending moves for the old state even though the server has
(rightfully so) already sent the next state query.

Example of an impatient server/slow client (unrealistically short
game):

	s: kgp 1 0 0
	c: mode simple
	s: state <...>
	s: stop
	s: state <...>
	s: stop
	s: state <...>
	s: stop
	c: yield
	c: yield
	c: yield
	s: state <...>
	s: stop
	c: move 1
	s: state <...>
	s: stop
	s: state <...>
	s: stop
	c: yield
	c: move 2
	c: move 2
	c: yield
	c: move 3
	c: yield
	s: goodbye
