import info.kwarc.kalah.Agent;
import info.kwarc.kalah.KalahState;
import info.kwarc.kalah.KalahState.GameResult;
import info.kwarc.kalah.KalahState.Player;
import info.kwarc.kalah.ProtocolManager;

import java.io.IOException;


// agent using min max search
class MinMaxAgent extends Agent {

    private final int level; // search depth

    public MinMaxAgent(String host, Integer port, ProtocolManager.ConnectionType conType, int level) {

        // TODO enter your data
        super(
                host,
                port,
                conType,
                "MinMax " + level,
                null, // authors go here
                null, // description goes here
                null, // token goes here
                true
        );

        this.level = level;
    }

    @Override
    public void search(KalahState ks) throws IOException {

        // submit some move so there is one in case we're running out of time
        submitMove(ks.randomLegalMove());

        // iterative deepening
        for (int max_depth = 1; max_depth <= level; max_depth++) {

            Integer eval = searchHelper(0, max_depth, ks);

            if (eval == null) {
                break; // search has been aborted
            } else if (eval == Integer.MAX_VALUE || eval == Integer.MIN_VALUE + 1) {
                break; // successive searches would get the same result -> yield
            }
        }
    }

    // Returns score from the player to move's point of view or null if it was aborted
    private Integer searchHelper(int depth, int max_depth, KalahState ks) throws IOException {

        // should search be aborted?
        if (shouldStop()) {
            return null;
        }

        // Is game over/the result determined (more than half of all seeds in one store)?
        GameResult result = ks.result();
        if (result != GameResult.UNDECIDED) {
            // Just terminate here, we already sent a legal move
            if (result == GameResult.KNOWN_WIN ||
                    result == GameResult.WIN) {
                return Integer.MAX_VALUE;
            } else if (result == GameResult.KNOWN_LOSS ||
                    result == GameResult.LOSS) {
                return Integer.MIN_VALUE + 1;
            } else {
                return 0;
            }
        } else {
            // reached max. depth but the position is non-terminal?
            if (depth == max_depth) {
                // evaluate
                return ks.getStoreLead();
            } else {
                // Otherwise continue recursively

                // Keep track of the best move and it's eval
                Integer best_move = null;
                Integer best_eval = null;

                // check all moves
                for (Integer move : ks.getMoves()) {
                    // copy the state to make the move, undoing moves in Kalah is annoying
                    KalahState copy = new KalahState(ks);

                    // Execute the move while keeping track of whether there's a change of turns
                    Player before = copy.getSideToMove();

                    copy.doMove(move);

                    Integer eval = searchHelper(depth + 1, max_depth, copy);

                    // should_stop() returned true --> abort search immediately
                    if (eval == null) {
                        return null;
                    }

                    // Other player's turn? Change sign of eval
                    if (before != copy.getSideToMove()) {
                        eval = -eval;
                    }

                    // Better eval than best so far? Make a note of that!
                    if (best_eval == null || eval > best_eval) {
                        best_eval = eval;
                        best_move = move;
                    }
                }

                // Root call? Tell the server about the move and comment about it
                if (depth == 0) {

                    // mandatory
                    submitMove(best_move);

                    String comment = "Best move: " + (best_move + 1) + "\n" +
                            "Eval: " + best_eval + "\n" +
                            "Depth: " + max_depth;

                    // optional
                    sendComment(comment);
                }

                return best_eval;
            }
        }
    }

    public static void main(String[] args) {

        while (true) {

            Agent agent = new MinMaxAgent(
                    "kalah.kwarc.info/socket",
                    null,
                    ProtocolManager.ConnectionType.WebSocketSecure,
                    5);

            try {
                agent.run();
            } catch (IOException e) {
                e.printStackTrace();
            }

            try {
                Thread.sleep(10_000);
            } catch (InterruptedException e) {
                e.printStackTrace();
            }
        }
    }

}