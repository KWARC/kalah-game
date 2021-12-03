Formal Structure
----------------

The protocol consists of commands sent between client and
server. Server-to-client and client-to-server commands have the same
form, consisting of:

* Optional, unique command ID.  Client and server MUST ensure that
  no ID is reused.
* Optional command reference (addressing a previous command ID).  The
  client MAY NOT reference a non-existing command ID.
* A command name
* A number of arguments

The ABNF representation of a command is as follows:

	command  = id name *(*1WSP argument) CRLF
	id       = [[*1DIGIT] [ref] *1WSP]
	ref      = ["@" *1DIGIT]
	name     = *1(DIGIT / ALPHA)
	argument = integer / real / word
	         / string / board
	integer  = [("+" / "-")] *1DIGIT
	real     = [("+" / "-")] *DIGIT "." *1DIGIT
	word     = *1(DIGIT / ALPHA / "-" / ":")
	string   = DQUOTE scontent DQUOTE
	scontent = *("\" CHAR / NDQCHAR)
	board    = "<" *1DIGIT *("," *1DIGIT) ">"

where `NDQCHAR` is every `CHAR` except for double quotes, backslashes
and line breaks. Each command MUST at most be most 16384 characters
long, including trailing white space. Any line beyond that MAY be
ignored by a server.

An argument has a statically identifiable type, and is either an
integer (`32`, `+0`, `-100`, ...), a real-valued number (`0.0`,
`+3.141`, `-.123`, ...), a string (`single-word`, `"with
double quotes"`, `"like \" this"`, ...) or a board literal.

Board literals are wrapped in angled-brackets and consist of a an
array of positive, unsigned integers separated using commas. The first
number indicates the board size $n$, the second and third give the
number of stones in the south and north Kalah respectively. Values 4 to
$4 + n$ list the number of stones in the south pits, $4 + n + 1$ to
$4 + 2n + 1$ the number of stones in the north pits:

    <3,10,2,1,2,3,4,2,0>
     ^ ^  ^ ^     ^
     | |  | |     |
     | |  | |     \__ North pits: 4, 2 and 0
     | |  | \________ South pits: 2, 1 and 3
     | |  \__________ North Kalah
     | \_____________ South Kalah
	 \_______________ Board Size
