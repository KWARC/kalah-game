#!/usr/bin/env python3

import kgp


def evaluation(state):
    return state[kgp.NORTH] - state[kgp.SOUTH]


def search(state, depth, side):
    if depth <= 0:
        return state

    choose = None
    if side == kgp.NORTH:
        choose = max
    elif side == kgp.SOUTH:
        choose = min

    def child(state, move):
        result, again = state.sow(side, move)
        if again:
            return search(result, depth-1, side)
        else:
            return search(result, depth-1, not side)

    return choose([child(state, move)
                   for move in state.legal_moves(side)],
                  key=evaluation)


def minmax_agent(state):
    for depth in iter(int, 1):
        yield search(state, depth, kgp.NORTH)


if __name__ == "__main__":
    kgp.connect(minmax_agent)
