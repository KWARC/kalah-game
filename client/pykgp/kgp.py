# KALAH GAME PROTOCOL LIBRARY                    -*- mode: python; -*-

# Copyright 2021, Philip Kaludercic

# Permission to use, copy, modify, and/or distribute this software for
# any purpose with or without fee is hereby granted, provided that the
# above  copyright notice  and this  permission notice  appear in  all
# copies.

# THE  SOFTWARE IS  PROVIDED  "AS  IS" AND  THE  AUTHOR DISCLAIMS  ALL
# WARRANTIES  WITH  REGARD  TO  THIS SOFTWARE  INCLUDING  ALL  IMPLIED
# WARRANTIES OF  MERCHANTABILITY AND  FITNESS. IN  NO EVENT  SHALL THE
# AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL
# DAMAGES OR ANY  DAMAGES WHATSOEVER RESULTING FROM LOSS  OF USE, DATA
# OR PROFITS,  WHETHER IN AN  ACTION OF CONTRACT, NEGLIGENCE  OR OTHER
# TORTIOUS ACTION,  ARISING OUT OF  OR IN  CONNECTION WITH THE  USE OR
# PERFORMANCE OF THIS SOFTWARE.

import inspect
import re
import os
import sys
import socket
import threading
import copy

_BOARD_PATTERN = re.compile(r'^(<(\d+(?:,\d+){4,})>)\s*')


NORTH = True
SOUTH = not NORTH


class Board:
    @staticmethod
    def parse(raw):
        """Turn a KGP board representation into a board object."""
        match = _BOARD_PATTERN.match(raw)
        assert match

        try:
            data = [int(d) for d in match.group(2).split(',')]
            size, north, south, *rest = data
            if len(data) != data[0] * 2 + 2 + 1:
                return None

            return Board(north, south, rest[:size], rest[size:])
        except ValueError:
            return None

    def __init__(self, north, south, north_pits, south_pits):
        """Create a new board."""
        assert len(north_pits) == len(south_pits)

        self.north = north
        self.south = south
        self.north_pits = north_pits
        self.south_pits = south_pits
        self.size = len(north_pits)

    def __str__(self):
        """Return board in KGP board representation."""
        data = [self.size,
                self.north,
                self.south,
                *self.north_pits,
                *self.south_pits]

        return '<{}>'.format(','.join(map(str, data)))

    def __getitem__(self, key):
        if key == NORTH:
            return self.north
        elif key == SOUTH:
            return self.south
        side, pit = key
        return self.pit(side, pit)

    def __setitem__(self, key, value):
        if key == NORTH:
            self.north = value
        elif key == SOUTH:
            self.south = value
        else:
            side, pit = key
            self.side(side)[pit] = value

    def side(self, side):
        """Return the pits for SIDE."""
        assert side in (NORTH, SOUTH)

        if side == NORTH:
            return self.north_pits
        elif side == SOUTH:
            return self.south_pits

    def pit(self, side, pit):
        """Return number of seeds in PIT on SIDE."""
        assert 0 <= pit < self.size
        return self.side(side)[pit]

    def is_legal(self, side, move):
        """Check if side can make move."""
        return self.pit(side, move) > 0

    def legal_moves(self, side):
        """Return a list of legal moves for side."""
        return [move for move in range(self.size)
                if self.is_legal(side, move)]

    def is_final(self):
        return (not self.legal_moves(NORTH)) or (not self.legal_moves(SOUTH))

    def copy(self):
        """Return a deep copy of the current board state."""
        return copy.deepcopy(self)

    def sow(self, side, pit, pure=True):
        """
        Sow the stones from pit on side.

        By default, sow returns a new board object and a boolean to
        indicate a repeat move. To change the state of this object set
        the key pure to False.
        """
        b = self
        if pure:
            b = self.copy()

        assert b.is_legal(side, pit)

        me = side
        pos = pit + 1
        stones = b[side, pit]
        b[side, pit] = 0

        while stones > 0:
            if pos == self.size:
                if side == me:
                    b[me] += 1
                    stones -= 1
                side = not side
                pos = 0
            else:
                b[side] += 1
                pos += 1
                stones -= 1

        if pos == 0 and not me == side:
            return b, True
        elif side == me and pos > 0:
            last = pos - 1
            other = self.size - 1 - last
            if b[side, last] == 1:
                b[side] += b[not side, other] + 1
                b[not side, other] = 0
                b[side] = 0

        return b, False


def connect(agent, host='localhost', port=2671, token=None, name=None, authors=[], debug=False):
    """
    Connect to KGP server at host:port as agent.

    Agent is a generator function, that produces as many moves as it
    can until the server ends a search request.

    The optional arguments TOKEN, NAME and AUTHORS are used to send
    the server optional information about the client implementation.

    If DEBUG has a true value, the network communication is printed on
    to the standard error stream.
    """
    assert inspect.isgeneratorfunction(agent)

    COMMAND_PATTERN = re.compile(r"""
^                   # beginning of line
\s*                 # preceding white space is ignored
(?:                 # the optional ID segment
(?P<id>\d+)         # ... must consist of an ID
(?:@(?P<ref>\d+))?  # ... and may consist of a reference
\s+                 # ... and must have trailing white space
)?
(?P<cmd>\w+)        # the command is just a alphanumeric word
(?:                 # the optional argument segment
\s+                 # ... ignores preceding white space
(?P<args>.*?)       # ... and matches the rest of the line
)?
\s*$                # trailing white space is ignored
""", re.VERBOSE)

    STRING_PATTERN = re.compile(r'^"((?:\\.|[^"])*)"\s*')
    INTEGER_PATTERN = re.compile(r'^(\d+)\s*')
    FLOAT_PATTERN = re.compile(r'^(\d+(?:\.\d+)?)\s*')
    BOARD_PATTERN = _BOARD_PATTERN

    def split(args):
        """
        Parse ARGS as far as possible.

        Returns a list of python objects, each equivalent to the
        elements of ARGS as parsed in order.
        """
        upto = 0
        parsed = []

        while True:
            for pat in (STRING_PATTERN,
                        INTEGER_PATTERN,
                        FLOAT_PATTERN,
                        BOARD_PATTERN):
                match = pat.search(args[upto:])
                if not match:
                    continue

                arg = match.group(1)
                if pat == STRING_PATTERN:
                    parsed.append(re.sub(r'\\(.)', '\\1', arg))
                elif pat == INTEGER_PATTERN:
                    parsed.append(int(arg))
                elif pat == FLOAT_PATTERN:
                    parsed.append(float(arg))
                elif pat == BOARD_PATTERN:
                    parsed.append(Board.parse(arg))
                else:
                    assert(False)

                upto += match.end(0)
                break
            else:
                return parsed

    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.setblocking(True)
        sock.connect((host, port))
        with sock.makefile(mode='rw') as pseudo:
            lock = threading.Lock()
            id = 1

            def send(cmd, *args, ref=None):
                """
                Send cmd with args to server.

                If ref is not None, add a reference.
                """
                nonlocal id

                msg = str(id)
                if ref:
                    msg += f'@{ref}'
                msg += " " + cmd

                for arg in args:
                    msg += " "
                    if isinstance(arg, str):
                        string = re.sub(r'"', '\\"', arg)
                        msg += f'"{string}"'
                    else:
                        msg += str(arg)

                if debug:
                    print(">", msg, file=sys.stderr)
                pseudo.write(msg + "\r\n")

                with lock:
                    id += 2

            def query(state, timeout, cid):
                """
                Start querying agent what move to make.

                State is the current board state, timeout a
                threading.Event object to indicate when the search
                time is over and cid the ID of the state command that
                issued the request.
                """
                if state.is_final():
                    return
                for move in agent(state):
                    if not type(move) is int:
                        raise TypeError("Not a move")
                    if timeout.is_set():
                        break
                    if move:
                        send("move", move, ref=cid)
                else:
                    send("yield", ref=cid)

            running = {}

            for line in pseudo:
                if debug:
                    print("<", line.strip(), file=sys.stderr)

                try:
                    match = COMMAND_PATTERN.match(line)
                    if not match:
                        continue
                    cid, ref = None, None
                    if match.group('id'):
                        cid = int(match.group('id'))
                    if match.group('ref'):
                        ref = int(match.group('ref'))
                    cmd = match.group('cmd')
                    args = split(match.group('args'))

                    if cmd == "kgp":
                        major, _minor, _patch = args
                        if major != 1:
                            send("error", "protocol not supported", ref=cid)
                            raise ValueError()
                        send("mode", "freeplay")
                        if name:
                            send("set", "info:name", name)
                        if authors:
                            send("set", "info:authors", ",".join(authors))
                        if token:
                            send("set", "auth:token", token)
                    elif cmd == "state":
                        board = args[0]

                        assert cid not in running

                        timeout = threading.Event()
                        thread = threading.Thread(
                            name='query-{}'.format(cid),
                            args=(board, timeout, cid),
                            target=query,
                            daemon=True)

                        running[cid] = timeout
                        thread.start()
                    elif cmd == "stop":
                        if ref and ref in running:
                            timeout = running[ref]
                            timeout.set()
                            running.pop(ref, None)
                    elif cmd == "ok":
                        pass    # ignored
                    elif cmd == "error":
                        pass    # ignored
                    elif cmd == "fail":
                        return
                    elif cmd == "ping":
                        if len(args) >= 1:
                            send("pong", args[0], ref=cid)
                        else:
                            send("pong", ref=cid)

                except ValueError:
                    pass
                except TypeError:
                    pass

# Local Variables:
# indent-tabs-mode: nil
# tab-width: 4
# End:
