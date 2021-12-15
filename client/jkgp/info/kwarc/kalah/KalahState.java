package info.kwarc.kalah;

import java.util.ArrayList;
import java.util.Arrays;

// A Kalah implementation with a sideToMove variable
// I am aware that you could do a recursive search by flipping the board accordingly
// but there is little performance gain, and it's a bit weird to be honest
// Ready to be used with a HashMap
// In a game over state, all seeds are in the stores, there's no seed in any house

public class KalahState {

    // arrays are sowed in the direction of increasing indices
    private int[] housesSouth, housesNorth;
    private int storeSouth, storeNorth;
    private Player playerToMove;
    // create a new board of size h with seeds seeds everywhere and south to move
    public KalahState(int board_size, int seeds) {
        storeSouth = 0;
        storeNorth = 0;
        housesSouth = new int[board_size];
        Arrays.fill(housesSouth, seeds);
        housesNorth = new int[board_size];
        Arrays.fill(housesNorth, seeds);
        playerToMove = Player.SOUTH;
    }
    // creates a copy of an existing KalahState
    public KalahState(KalahState state) {
        storeSouth = state.storeSouth;
        storeNorth = state.storeNorth;
        housesSouth = Arrays.copyOf(state.housesSouth, state.housesSouth.length);
        housesNorth = Arrays.copyOf(state.housesNorth, state.housesNorth.length);
        playerToMove = state.playerToMove;
    }

    // mirrors the board, switches stores, houses and side to move
    public void flip() {
        int tmp = storeSouth;
        storeSouth = storeNorth;
        storeNorth = tmp;

        int[] tmp2 = housesSouth;
        housesSouth = housesNorth;
        housesNorth = tmp2;

        playerToMove = playerToMove.other();
    }

    // helper function to flip back the state, see flipIfNorthToMove()
    public void flipIfWasFlipped(boolean wasFlipped) {
        if (wasFlipped) {
            flip();
        }
    }

    // Helper function to make sure it's south to move
    // Returns whether it was flipped.
    // That boolean can be used by flipIfWasFlipped() later on for comfortable flipping
    // That way functions only need to be implemented from south's perspective
    // I have a lot of trust in Java's optimization ^^
    public boolean flipIfNorthToMove() {
        if (playerToMove == Player.NORTH) {
            flip();
            return true;
        }
        return false;
    }

    // returns the 'leftmost' legal move, the one with the lowest array index
    // useful if you just need any legal move
    public int lowestLegalMove() {
        boolean wasFlipped = flipIfNorthToMove();

        int llm = -1;
        for (int i = 0; i < getBoardSize(); i++) {
            if (housesSouth[i] != 0) {
                llm = i;
                break;
            }
        }

        flipIfWasFlipped(wasFlipped);
        return llm;
    }

    public int randomLegalMove() {
        boolean wasFlipped = flipIfNorthToMove();

        ArrayList<Integer> moves = getMoves();
        int randomIndex = (int) (Math.random() * moves.size());
        int chosenMove = moves.get(randomIndex);

        flipIfWasFlipped(wasFlipped);
        return chosenMove;
    }

    // returns the number of legal moves, useful if your evaluation is based on that value
    public int numberOfMoves() {
        boolean wasFlipped = flipIfNorthToMove();

        int c = 0;
        for (int i = 0; i < getBoardSize(); i++) {
            if (housesSouth[i] != 0) {
                c++;
            }
        }

        flipIfWasFlipped(wasFlipped);
        return c;
    }

    // painfully slow, better iterate through the moves yourself
    public ArrayList<Integer> getMoves() {
        boolean wasFlipped = flipIfNorthToMove();

        ArrayList<Integer> moves = new ArrayList<>(numberOfMoves());
        for (int i = 0; i < getBoardSize(); i++) {
            if (housesSouth[i] != 0) {
                moves.add(i);
            }
        }

        flipIfWasFlipped(wasFlipped);
        return moves;
    }

    // returns true if the player is allowed to move again after the given move
    public boolean isDoubleMove(int move) {
        boolean wasFlipped = flipIfNorthToMove();
        int endsUp = (move + housesSouth[move]) % (2 * getBoardSize() + 1);
        flipIfWasFlipped(wasFlipped);
        return endsUp == getBoardSize();
    }

    // returns true if the given move is a capture move
    public boolean isCaptureMove(int move) {
        boolean wasFlipped = flipIfNorthToMove();

        int m = move;
        int s = getBoardSize();

        // index of pit the last seed will end up in
        // (imagining that the indices would continue in sowing direction after the last southern house)
        int endsUp = (m + housesSouth[move]) % (2 * s + 1);

        // capture move must drop last seed in southern pit
        if (endsUp >= s) {
            return false;
        }

        // how many stones in the pit opposite to where the last seed is dropped?
        int op = housesNorth[s - 1 - endsUp];

        // did we add a seed to that pit before capturing it?
        if (m + housesSouth[m] >= s) {
            op++;
        }

        boolean b;

        // Played one around the board? Then there's no empty pit to capture with
        if (m + housesSouth[m] > 2 * s + 1 ||

                // Either the pit where the last seeds drops was empty at the start
                // or it's the starting pit (which we emptied at the start of our move)
                (endsUp != m && housesSouth[endsUp] != 0) ||
                // The opposite pit has to contain at least one seed
                op == 0) {
            b = false;
        } else {
            b = true;
        }

        flipIfWasFlipped(wasFlipped);

        return b;
    }

    // executes the given move
    public void doMove(int move) {
        boolean wasFlipped = flipIfNorthToMove();

        // sow the seeds

        // grab the seeds
        int hand = housesSouth[move];
        housesSouth[move] = 0;
        int pos = move;
        boolean sowSouth = true;

        // sow until done
        while (hand != 0) {
            // pos < ... --> houses
            // pos = ... --> store
            // pos > ... --> switch to other side

            pos++;
            hand--;

            // skip northern store
            if (sowSouth && pos > getBoardSize()) {
                pos = 0;
                sowSouth = false;
            } else if (!sowSouth && pos > getBoardSize() - 1) {
                pos = 0;
                sowSouth = true;
            }

            if (sowSouth) {
                if (pos < getBoardSize()) {
                    housesSouth[pos]++;
                } else if (pos == getBoardSize()) {
                    storeSouth++;
                }
            } else {
                if (pos < getBoardSize()) {
                    housesNorth[pos]++;
                }
            }
        }

        // handle turn
        if (pos != getBoardSize()) // last seed in store?
        {
            playerToMove = Player.NORTH;
        }

        // handle captures

        // get corresponding northern house
        int cnh = getBoardSize() - 1 - pos;
        // last seed in southern house and seeds in opponents house

        if (sowSouth && pos < getBoardSize() && housesSouth[pos] == 1 && housesNorth[cnh] != 0) {
            storeSouth += housesSouth[pos];
            housesSouth[pos] = 0;
            storeSouth += housesNorth[cnh];
            housesNorth[cnh] = 0;
        }

        // handle one side being empty
        cleanUpOneSidedHouses();

        flipIfWasFlipped(wasFlipped);
    }

    // returns all seeds, both houses and both stores
    public int totalSeeds() {
        return storeSouth + storeNorth + getHouseSumSouth() + getHouseSumSouth();
    }

    // result from the player to move's perspective
    public GameResult result() {
        boolean wasFlipped = flipIfNorthToMove();

        GameResult r;

        if (getHouseSumSouth() == 0) // game over
        {
            if (storeSouth > storeNorth)
                r = GameResult.WIN;
            else if (storeSouth == storeNorth)
                r = GameResult.DRAW;
            else
                r = GameResult.LOSS;
        } else // not game over
        {
            if (storeSouth > totalSeeds() / 2) // south is going to win no matter what
                r = GameResult.KNOWN_WIN;
            else if (storeNorth > totalSeeds() / 2) // north is going to win no matter what
                r = GameResult.KNOWN_LOSS;
            else
                r = GameResult.UNDECIDED;
        }

        flipIfWasFlipped(wasFlipped);
        return r;
    }

    // Move seeds of a players houses to that players store if opponent has no seeds on his side
    public void cleanUpOneSidedHouses() {
        if (getHouseSumSouth() == 0) {
            for (int i = 0; i < getBoardSize(); i++) {
                storeNorth += housesNorth[i];
                housesNorth[i] = 0;
            }
        } else if (getHouseSumNorth() == 0) {
            for (int i = 0; i < getBoardSize(); i++) {
                storeSouth += housesSouth[i];
                housesSouth[i] = 0;
            }
        }
    }

    // returns store diff from player to move's point of view
    public int getStoreLead() {
        if (playerToMove == Player.SOUTH) {
            return storeSouth - storeNorth;
        } else {
            return storeNorth - storeSouth;
        }
    }

    // returns the number of houses on one side
    public int getBoardSize() {
        return housesSouth.length;
    }

    // returns the side to move
    public Player getSideToMove() {
        return playerToMove;
    }

    // get the number of seeds in south's store
    public int getStoreSouth() {
        return storeSouth;
    }

    // set the number of seeds in south's store
    public void setStoreSouth(int seeds) {
        storeSouth = seeds;
    }

    // get the number of seeds in north's store
    public int getStoreNorth() {
        return storeNorth;
    }

    // set the number of seeds in north's store
    public void setStoreNorth(int seeds) {
        storeNorth = seeds;
    }

    // set the number of seeds of the given house
    // indices start at 0 and increase in the direction of sowing
    public void setHouse(Player player, int index, int seeds) {
        if (player == Player.SOUTH) {
            housesSouth[index] = seeds;
        } else {
            housesNorth[index] = seeds;
        }
    }

    // get the number of seeds of the given house
    // indices start at 0 and increase in the direction of sowing
    public int getHouse(Player player, int index) {
        if (player == Player.SOUTH) {
            return housesSouth[index];
        } else {
            return housesNorth[index];
        }
    }

    // returns the sum of all southern houses
    public int getHouseSumSouth() {
        int sum = 0;
        for (int p : housesSouth) {
            sum += p;
        }
        return sum;
    }

    // returns the sum of all northern houses
    public int getHouseSumNorth() {
        int sum = 0;
        for (int p : housesNorth) {
            sum += p;
        }
        return sum;
    }

    // returns the sum of all north's and south's houses
    public int getHouseSum() {
        return getHouseSumSouth() + getHouseSumNorth();
    }

    // creating a hash code, not considering the side to move, but that's ok
    // (if two states are equal THEN the hash code must be equal, not the other way around)
    @Override
    public int hashCode() {
        int ah = Arrays.hashCode(housesSouth) ^ Arrays.hashCode(housesNorth);
        int sh = Integer.hashCode(storeSouth ^ Integer.hashCode(storeNorth));
        return ah ^ sh;
    }

    // checks if two states are equal regarding the side to move, the stores and the houses
    @Override
    public boolean equals(Object o) {

        if (!(o instanceof KalahState)) {
            return false;
        }

        KalahState s = (KalahState) o;

        return playerToMove == s.playerToMove &&
                storeSouth == s.storeSouth &&
                storeNorth == s.storeNorth &&
                Arrays.equals(housesSouth, s.housesSouth) &&
                Arrays.equals(housesNorth, s.housesNorth);
    }

    // returns a nice multiline representation which includes the side to move
    @Override
    public String toString() {
        StringBuilder hn = new StringBuilder("\t");
        StringBuilder hs = new StringBuilder("\t");
        StringBuilder ss = new StringBuilder(storeNorth + "\t");
        for (int i = 0; i < getBoardSize(); i++) {
            hn.append(housesNorth[getBoardSize() - 1 - i]).append("\t");
            hs.append(housesSouth[i]).append("\t");
            ss.append('\t');
        }
        ss.append(storeSouth);
        ss.append("\t");
        ss.append(playerToMove == Player.SOUTH ? "South" : "North");


        return hn.toString() + '\n' + ss + '\n' + hs;
    }

    public enum GameResult {
        UNDECIDED, // game could still go either way

        LOSS, // game over, player to move has less in his store
        DRAW, // game over, both stores have the same number of seeds
        WIN,  // game over, player to move has more seeds in his store

        KNOWN_LOSS, // not game over, but result will be a LOSS no matter what happens
        KNOWN_WIN, // not game over, but result will be a WIN no matter what happens
    }

    public enum Player {
        NORTH,
        SOUTH;

        public Player other() {
            if (this == NORTH) {
                return SOUTH;
            } else {
                return NORTH;
            }
        }
    }
}
