Protocol Overview
-----------------

The communication MUST begin by the server sending the client a `kgp`
command, with three arguments indicating the major, minor and patch
version of the implemented protocol, e.g.:

	kgp 1 0 1
	
The client MUST parse this command and that it implements everything
that is necessary to communicate. The major version indicates
backwards incompatible changes, the minor version indicates forwards
incompatible changes and the patch version indicates minor changes. A
client MAY only check the major version to ensure compatibility, and
MUST check the minor and patch version to ensure availability of later
improvements to the protocol.

The client MUST eventually proceed to respond with a `mode` command,
indicating the activity it is interested in. The `mode` command is
REQUIRED to have one string-argument, indicating the activity.

	mode freeplay

In case the server doesn't recognize or support the requested
activity, it MUST immediately indicate an error and close the
connection:

	error "Unsupported activity"
	goodbye

The detail of how the protocol continues depends on the chosen
activity. The server SHOULD terminate the connection with a `goodbye`
command.

After the connection has been established and version compatibility
has been ensured, the server MAY send a `ping` command. The
client MUST answer with `pong`, and SHOULD do so as quickly as
possible. In absence of a response, the server SHOULD terminate the
connection.

Both client and server MAY send `set` commands give the other party
hints. Both client and server SHOULD try to handle these, but MUST NOT
terminate the connection because of an unknown option. Version
commands indicating capabilities and requests SHOULD be handled
between the version compatibility is ensured (`kgp`) and the activity
request (`mode`).

Any command (client or server) MAY be referenced by a response
command: `ok` for confirmations and `error` for to indicate an illegal
state or data. All three MUST give a semantically-opaque string
argument. The interpretation of a response depends on the mode.
