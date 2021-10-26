#!/usr/bin/env python3

import kgp
from random import choice


def random_agent(state):
    try:
        yield choice(state.legal_moves(kgp.NORTH))
    except IndexError:
        pass


if __name__ == "__main__":
    kgp.connect(random_agent)
