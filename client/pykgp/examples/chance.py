#!/usr/bin/env python3

import os
import random
import time

import kgp

def random_agent(state):
    yield random.choice(state.legal_moves(kgp.SOUTH))


if __name__ == "__main__":
    kgp.connect(random_agent, host="localhost", token=os.getenv("TOKEN"), name=os.getenv("NAME"), debug=True)
