#!/usr/bin/env python3

from kgp import *
import unittest


class TestBoard(unittest.TestCase):
    def test_eq(self):
        self.assertEqual(Board(0,0,[],[]), Board(0,0,[],[]))
        self.assertEqual(Board(1,0,[],[]), Board(1,0,[],[]))
        self.assertNotEqual(Board(1,0,[],[]), Board(0,1,[],[]))
        self.assertEqual(Board(4,5,[1,2,3],[6,7,8]),
                         Board(4,5,[1,2,3],[6,7,8]))


    def test_parse(self):
        self.assertEqual(Board.parse("<1,1,1,1,1>"), Board(1,1,[1],[1]))
        self.assertEqual(Board.parse("<3,4,5,1,2,3,6,7,8>"),
                         Board(4,5,[1,2,3],[6,7,8]))

    def test_str(self):
        self.assertEqual("<0,0,0>", str(Board(0,0,[],[])))
        self.assertEqual("<1,1,1,1,1>", str(Board(1,1,[1],[1])))
        self.assertEqual("<3,4,5,1,2,3,6,7,8>", str(Board(4,5,[1,2,3],[6,7,8])))


    def test_access(self):
        b = Board(4,5,[1,2,3],[6,7,8])
        self.assertEqual(b[SOUTH], 4)
        self.assertEqual(b[NORTH], 5)
        self.assertEqual(b[SOUTH, 1], 2)
        self.assertEqual(b[NORTH, 0], 6)


    def test_is_legal(self):
        b = Board(4,5,[0,2,3],[6,7,8])
        self.assertTrue(b.is_legal(NORTH, 0))
        self.assertTrue(b.is_legal(NORTH, 1))
        self.assertFalse(b.is_legal(SOUTH, 0))
        self.assertTrue(b.is_legal(SOUTH, 1))


    def test_is_legal(self):
        b = Board(4,5,[0,2,3],[6,7,8])
        self.assertTrue(b.is_legal(NORTH, 0))
        self.assertTrue(b.is_legal(NORTH, 1))
        self.assertFalse(b.is_legal(SOUTH, 0))
        self.assertTrue(b.is_legal(SOUTH, 1))


    def test_is_final(self):
        self.assertFalse(Board(4,5,[0,2,3],[6,7,8]).is_final())
        self.assertFalse(Board(4,5,[0,0,3],[6,7,8]).is_final())
        self.assertTrue(Board(4,5,[0,0,0],[6,7,8]).is_final())


    def test_sow(self):
        self.assertEqual(
            Board(0,0,[3,3,3],[3,3,3]).sow(NORTH, 0),
            (Board(0,1,[3,3,3],[0,4,4]), True))
        self.assertEqual(
            Board(0,0,[5,5,5,5],[5,5,5,5]).sow(NORTH, 2),
            (Board(0,1,[6,6,6,5],[5,5,0,6]), False))
        self.assertEqual(
            Board(0,0,[3,3,3],[3,3,3]).sow(NORTH, 2),
            (Board(0,1,[4,4,3],[3,3,0]), False))
        self.assertEqual(
            Board(1,1,[3,3,3],[3,3,3]).sow(SOUTH, 1),
            (Board(2,1,[3,0,4],[4,3,3]), False))
        self.assertEqual(
            Board(0,0,[9,9,9],[9,9,9]).sow(NORTH, 0),
            (Board(0,1,[10,10,10],[1,11,11]), False))
        self.assertEqual(
            Board(0,0,[9,9,9],[9,9,9]).sow(NORTH, 0),
            (Board(0,1,[10,10,10],[1,11,11]), False))
        self.assertEqual(
            Board(0,0,[0,0,1],[1,1,1]).sow(SOUTH, 2),
            (Board(1,3,[0,0,0],[0,0,0]), False))
        self.assertEqual(
            Board(0,0,[0,0,2],[1,1,1]).sow(SOUTH, 2),
            (Board(1,4,[0,0,0],[0,0,0]), False))
        self.assertEqual(
            Board(0,0,[1,0,0],[1,1,0]).sow(NORTH, 1),
            (Board(0,3,[0,0,0],[0,0,0]), False))


if __name__ == '__main__':
    unittest.main()
