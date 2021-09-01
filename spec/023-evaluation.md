Evaluation Mode
---------------

The "evaluation" mode involves the client giving numerical evaluations
for given states. An evaluation is a real-valued number, without any
specified meaning. The client SHOULD be consistent in evaluating
states (the same board should be approximately equal, a board with a
better chance of winning should have a better score, ...).

After requesting the mode with

	mode eval
	
the server may immediately start by sending `state` commands as
specified for the "freeplay" mode.

Evaluation commands
-------------------

`state [board]` (server)

: See "Freeplay commands". The server MUST send a command ID.

`eval [real]` (client)

: The client MUST reference the ID of the `state` command it is
  evaluating. Multiple commands can be sent out in reference to one
  `state` request.
  
`stop` (client)

: See "Freeplay commands". The server MUST use a command
  reference. The client SHOULD stop responding to the referenced
  `state` request.
