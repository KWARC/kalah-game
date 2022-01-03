package info.kwarc.kalah;

import java.util.ArrayList;
import java.util.Arrays;

/**
 * A Kalah implementation with a sideToMove variable.
 * We are aware that one could also flip the board accordingly but went for former since it's easier to use.
 * Moves are indexed from 0 to n-1 in sowing direction where n is the number of southern pits
 * Note that this implementation is also ready for use with HashMaps.
 */
public class KalahState {

    // arrays are sowed in the direction of increasing indices
    private int[] housesSouth, housesNorth;
    private int storeSouth, storeNorth;
    private Player playerToMove;

    /**
     * Creates a board of @param board_size pits with @param seeds each.
     * @param board_size Number of southern pits
     * @param seeds Number of initial seeds in one pit
     */
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

    /**
     * Creates a copy of the given Board
     */
    public KalahState(KalahState state) {
        storeSouth = state.storeSouth;
        storeNorth = state.storeNorth;
        housesSouth = Arrays.copyOf(state.housesSouth, state.housesSouth.length);
        housesNorth = Arrays.copyOf(state.housesNorth, state.housesNorth.length);
        playerToMove = state.playerToMove;
    }

    /**
     * Mirrors the board e.g. flips stores, houses and side to move
     */
    public void flip() {
        int tmp = storeSouth;
        storeSouth = storeNorth;
        storeNorth = tmp;

        int[] tmp2 = housesSouth;
        housesSouth = housesNorth;
        housesNorth = tmp2;

        playerToMove = playerToMove.other();
    }

    /**
     * Useful helper function, so algorithms have to be implemented from souths point of view only.
     * Used together with flipIfNorthToMove()
     * @param wasFlipped Whether the board should be flipped
     */
    public void flipIfWasFlipped(boolean wasFlipped) {
        if (wasFlipped) {
            flip();
        }
    }

    /**
     * Flips the board if it's North's turn e.g. so that it's South's turn afterwards.
     * Useful helper function, so algorithms have to be implemented from souths point of view only.
     * Used together with flipIfWasFlipped()
     * @return true if the board was flipped
     */
    public boolean flipIfNorthToMove() {
        if (playerToMove == Player.NORTH) {
            flip();
            return true;
        }
        return false;
    }

    /**
     * @return Leftmost legal move e.g. with the lowest array index
     * Useful if you need any legal move
     */
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

    /**
     * @return Returns a random legal move
     */
    public int randomLegalMove() {
        boolean wasFlipped = flipIfNorthToMove();

        ArrayList<Integer> moves = getMoves();
        int randomIndex = (int) (Math.random() * moves.size());
        int chosenMove = moves.get(randomIndex);

        flipIfWasFlipped(wasFlipped);
        return chosenMove;
    }

    /**
     * @return Returns the number of legal moves
     */
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

    /**
     * @return ArrayList of legal moves
     */
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

    /**
     * @return True iff the last seed would end up in the store
     */
    public boolean isDoubleMove(int move) {
        boolean wasFlipped = flipIfNorthToMove();
        int endsUp = (move + housesSouth[move]) % (2 * getBoardSize() + 1);
        flipIfWasFlipped(wasFlipped);
        return endsUp == getBoardSize();
    }

    /**
     * @return True iff the move would be a capture
     */
    public boolean isCaptureMove(int move) {
        boolean wasFlipped = flipIfNorthToMove();

        int m = move;
        int s = getBoardSize();

        // index of pit the last seed will end up in
        // (imagining that the indices would continue in sowing direction after the last southern house)
        int endsUp = (m + housesSouth[move]) % (2 * s + 1);

        // capture move must drop last seed in southern pit
        if (endsUp >= s) {
            flipIfWasFlipped(wasFlipped);
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
        b = m + housesSouth[m] <= 2 * s + 1 &&

                // Either the pit where the last seeds drops was empty at the start
                // or it's the starting pit (which we emptied at the start of our move)
                (endsUp == m || housesSouth[endsUp] == 0) &&
                // The opposite pit has to contain at least one seed
                op != 0;

        flipIfWasFlipped(wasFlipped);

        return b;
    }

    /**
     * Executes the given move by modifying the board.
     * Note that there is a constructor for copying from an existing board.
     */
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

    /**
     * @return Sum of all seeds, from both stores and all houses
     */
    public int totalSeeds() {
        return storeSouth + storeNorth + getHouseSumSouth() + getHouseSumSouth();
    }

    /**
     * @return GameResult from the player to move's perspective
     */
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

    /**
     * If either northern or southern houses are empty e.g. the game is over
     * then the remaining seeds in southern houses are moved to the southern store
     * and the remaining seeds in northern houses are moved to the northern store
     */
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

    /**
     * @return Difference between stores from the perspective of the player who is about to move e.g.
     * positive if the player who is about to move has more seeds in their store
     */
    public int getStoreLead() {
        if (playerToMove == Player.SOUTH) {
            return storeSouth - storeNorth;
        } else {
            return storeNorth - storeSouth;
        }
    }

    /**
     * @return Returns the number of southern pits
     */
    public int getBoardSize() {
        return housesSouth.length;
    }

    /**
     * @return Returns the side to move
     */
    public Player getSideToMove() {
        return playerToMove;
    }

    /**
     * @return Number of seeds in the southern store
     */
    public int getStoreSouth() {
        return storeSouth;
    }

    /**
     * Set the number of seeds in the southern store
     */
    public void setStoreSouth(int seeds) {
        storeSouth = seeds;
    }

    /**
     * @return Number of seeds in the northern store
     */
    public int getStoreNorth() {
        return storeNorth;
    }

    /**
     * Set the number of seeds in the northern store
     */
    public void setStoreNorth(int seeds) {
        storeNorth = seeds;
    }

    /**
     * @param index index of the pit
     * Sets the number of seeds in the given house
     */
    public void setHouse(Player player, int index, int seeds) {
        if (player == Player.SOUTH) {
            housesSouth[index] = seeds;
        } else {
            housesNorth[index] = seeds;
        }
    }

    /**
     * @return Number of seeds in this pit
     */
    public int getHouse(Player player, int index) {
        if (player == Player.SOUTH) {
            return housesSouth[index];
        } else {
            return housesNorth[index];
        }
    }

    /**
     * @return Sum of seeds in southern houses
     */
    public int getHouseSumSouth() {
        int sum = 0;
        for (int p : housesSouth) {
            sum += p;
        }
        return sum;
    }

    /**
     * @return Sum of seeds in northern houses
     */
    public int getHouseSumNorth() {
        int sum = 0;
        for (int p : housesNorth) {
            sum += p;
        }
        return sum;
    }

    /**
     * @return Sum of seeds in all houses
     */
    public int getHouseSum() {
        return getHouseSumSouth() + getHouseSumNorth();
    }

    /**
     * @return Hash code for board, not considering who is about to move
     */
    @Override
    public int hashCode() {
        int ah = Arrays.hashCode(housesSouth) ^ Arrays.hashCode(housesNorth);
        int sh = Integer.hashCode(storeSouth ^ Integer.hashCode(storeNorth));
        return ah ^ sh;
    }

    /**
     * @return True iff stores, houses and turn are perfectly equal
     */
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

    /**
     * @return Multiline representation of the board, including stores and turn
     */
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


    /**
     * Enum for possible game outcomes
     */
    public enum GameResult {
        /**
         * The outcome of the game is not determined by the seeds in the stores
         */
        UNDECIDED,

        /**
         * All houses are empty, the other player has more seeds in their store
         */
        LOSS,

        /**
         * All houses are empty, both players have the same number of seeds in their stores
         */
        DRAW,

        /**
         * All houses are empty, the current player has more seeds in their store
         */
        WIN,

        /**
         * Not all houses are empty, but the other player has more than half of all seeds in their store,
         * so it's known that the current player will loose
         */
        KNOWN_LOSS,

        /**
         * Not all houses are empty, but the current player has more than half of all seeds in their store,
         * so it's known that the current player will win
         */
        KNOWN_WIN,
    }

    /**
     * The players are North and South
     */
    public enum Player {
        /**
         * The Northern/upper/Black player, moves second
         */
        NORTH,

        /**
         * The Southern/lower/White player, moves first
         */
        SOUTH;

        /**
         * @return Other player
         */
        public Player other() {
            if (this == NORTH) {
                return SOUTH;
            } else {
                return NORTH;
            }
        }
    }
}
