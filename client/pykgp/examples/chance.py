#!/usr/bin/env python3

import os
import random

import kgp

def random_agent(state):
    yield random.choice(state.legal_moves(kgp.SOUTH))


if __name__ == "__main__":
    kgp.connect(random_agent, token=os.getenv("TOKEN"))
