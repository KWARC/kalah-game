#!/usr/bin/env python3

import os
from random import choice

import kgp

def random_agent(state):
    try:
        yield choice(state.legal_moves(kgp.SOUTH))
    except IndexError:
        pass


if __name__ == "__main__":
    kgp.connect(random_agent, token=os.getenv("TOKEN"))
