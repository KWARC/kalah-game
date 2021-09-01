Protocol Overview
-----------------

The communication MUST begin by the server sending the client a `kgp`
command, with three arguments indicating the major, minor and patch
version of the implemented protocol, e.g.:

	kgp 1 0 0
	
The client MUST parse this command and that it implements everything
that is necessary to communicate. The server SHOULD NOT add a command
ID to the `kgp` command.

The client MUST eventually proceed to respond with a `mode` command,
indicating the activity it is interested in. The `mode` command is
REQUIRED to have one string-argument, indicating the activity.

	mode freeplay

In case the server doesn't recognize or support the requested
activity, it MUST immediately indicate an error and close the
connection:

	error "Unsupported activity"
	goodbye
	
Otherwise the server MUST eventually send an `init` command, to
indicate that an activity is about to begin. In absence of an error
message and an `init` response, the client SHOULD assume that the
server is processing its request.

The detail of how the protocol continues depends on the chosen
activity. The server SHOULD terminate the connection with a `goodbye`
command.

At any time the server MAY send a `ping` command. The client MUST
answer with `pong`, and SHOULD do so as quickly as possible. In
absence of a response, the server SHOULD terminate the connection.

Both client and server MAY send `set` commands give the other party
hints. Both client and server SHOULD try to handle these, but MUST NOT
terminate the connection because of an unknown option.

Any command (client or server) MAY be referenced by a response
command: `ok` for confirmations, `nok` for negations and `error` for
to indicate an illegal state or data. All three MUST give a
semantically-opaque string argument. The interpretation of a response
depends on the mode.
