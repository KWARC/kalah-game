#!/usr/bin/env python3

import kgp


def evaluate(state):
    return state[kgp.NORTH] - state[kgp.SOUTH]


def search(state, depth, side):
    def child(state, move):
        if depth <= 0:
            return (evaluate(state), move)

        after, again = state.sow(side, move)
        if after.is_final():
            return (evaluate(after), move)
        if again:
            return search(after, depth-1, side)
        else:
            return search(after, depth-1, not side)

    choose = max if side == kgp.NORTH else min
    return choose((child(state, move) for move in state.legal_moves(side)),
                  key=lambda ent: ent[0])


def agent(state):
    for depth in range(1, 16):
        yield search(state, depth, kgp.NORTH)[1]


if __name__ == "__main__":
    kgp.connect(agent)
