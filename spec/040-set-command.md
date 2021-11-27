The `set` Command
=================

The `set` command may be used at any time by both client and server to
inform the other side about capabilities, internal states, rules,
etc. The structure of a set command is

	set [option] [value]

Each option is structured using colons (`:`) to group commands
together. Each command group specified here SHOULD be implemented
entirely by both client and server:
