`game`-group
------------

As neither "freeplay" nor "simple" mode guarantee a logical sequence
of `state` commands, that might represent a possible game, the agent
cannot assume that two consecutive state commands represent the
chronological development of a game between the south and north sides.

In case the server internally matches two clients against one another,
and sends these a logical sequence of `state` commands, the `game`
group may be used to indicate this.

The options this groups offers are:

`game:id` (string)

: An opaque identifier to represent a logical game.  The option MUST
  be set before a `state` command has been sent.  The client MAY then
  associate `state` commands with the same `game:id` annotations and
  assume them to be a sequence of game states.

  Two consecutive state commands with the same `game:id` MUST
  represent two game states.  An empty string indicates an anonymous
  game.

`game:uri` (string)

: A URI pointing to a resource that describes the current game in more
  detail.  The resource should be publicly accessible, or provide the
  necessary credentials for the client to access it.

  An empty string indicates there is no URI for this game.

`game:opponent` (string)

: A name of the opponent the client is playing against.  The name
  SHOULD be unique.  Interpreted the same way `game:id` is.  An empty
  string indicates an unknown opponent.
