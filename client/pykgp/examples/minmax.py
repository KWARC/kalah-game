#!/usr/bin/env python3

# To write a Python client, we first need to import kgp.  In case
# "kgp.py" is not in the current working directory, adjust your
# PYTHONPATH.

import kgp

# We will be using a simple evaluation strategy: Compare the
# difference of stones in my (north) store vs. the opponents store
# (south).  kgp.py always assumes the agent is on the south side of
# the board, to avoid confusion.

def evaluate(state):
    return state[kgp.SOUTH] - state[kgp.NORTH]

# The following procedure implements the actual search.  We
# recursively traverse the search space, using EVALUATE from above to
# find our best/the opponents worst move.
#
# This is the actually interesting part, that you have to improve on.

def search(state, depth, side):
    def child(move):
        if depth <= 0:
            return (evaluate(state), move)

        after, again = state.sow(side, move)
        if after.is_final():
            return (evaluate(after), move)
        if again:
            return (search(after, depth, side)[0], move)
        else:
            return (search(after, depth-1, not side)[0], move)

    choose = max if side == kgp.SOUTH else min
    return choose((child(move) for move in state.legal_moves(side)),
                  key=lambda ent: ent[0])

# The actual agent is just a generator function, that is to say a
# function that when called will return a generator object.  The
# simplest way to implement this is to use the yield keyword.  In case
# you are not familiar with the keyword, take a look at the Python
# documentation:
#
#       https://docs.python.org/3/reference/expressions.html#yield
#
# The generator function takes a state, i.e. what the board currently
# looks like and yields a series of iterative improvements on what the
# best move should be.
#
# You can generate as many moves as you want, but after the server
# informs the client that the time is over, all following values by
# the generator are ignored.
#
# Note that the agent is invoked in a separate process that will be
# abruptly killed as soon as it's time has come.  By default you
# cannot share resources between invocation.
#
# If you need to synchronise state between searches, use the
# multiprocessing module (specifically multiprocessing.Value or
# multiprocessing.Array might be of interest).

def agent(state):
    for depth in range(1, 16):
        yield search(state, depth, kgp.SOUTH)[1]

# We can now use the generator function as an agent, as seen below.
#
# By default the client will connect to our training server, but you
# can change this by setting the "HOST" keyword:
#
#     kgp.connect(agent, host="localhost:2761")
#
# Note that by default kgp.py requires that the "websocket-client"
# library (not to be confused with "websockets", that ends with an
# "s") has to be installed for Python 3, as the public server is only
# accessible over a websocket connection.
#
# A more extensive example includes some agent metadata:
#
#     kgp.connect(agent, host    = "wss://kalah.kwarc.info/socket",
#                        token   = "A hopefully random string only I know",
#                        authors = ["Eva Lu Ator", "Ben Bitdiddle"],
#                        name    = "magenta")
#
# This will be sent to the server and used to identify a client over
# multiple connections.  You may leave out the TOKEN keyword, if you
# wish to stay anonymous, in which case your client will not appear in
# the leaderboard.

if __name__ == "__main__":
    import os
    kgp.connect(agent, host="localhost", debug=True, token=os.getenv("TOKEN"), name="Minmax 16")
