Verification Mode
-----------------

To ensure that clients don't misinterpret the rules of Kalah, they can
request this game mode and have the server challenge them with random
game states that they should compute.

A client does this by initially sending

	mode verify

Verification commands
---------------------

`problem [board] [move]` (server)

: The server send the valid board state and a legal move.  The client
  will respond to this using `solution`.  The server MUST send a
  command ID.

`solution [board] [integer]` (client)

: A response to a `problem`.  The client sends back the resulting
  board state and an indication whether or not the move was a repeat
  move or not (0 for false and non-0 for true).  The client SHOULD use
  a command ID.

  The server will respond to this message with an `erorr` message in
  case the client made a mistake.
