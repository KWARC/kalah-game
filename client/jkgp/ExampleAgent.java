import info.kwarc.kalah.Agent;
import info.kwarc.kalah.KalahState;
import info.kwarc.kalah.ProtocolManager;

import java.io.IOException;
import java.util.ArrayList;
import java.util.Random;

// Simple example of an agent
// Chooses among the legal moves uniform at random, sends new "best" moves in increasing intervals
// the latter is just to make it a better example, we don't want to provide any algorithms here
class ExampleAgent extends Agent {

    // Token could be any String, but PLEASE use a big non-empty(!!) randomized String for security's sake
    // and so we can distinguish your agents.
    // Concat together some random (decimal/hexadecimal) integers or headbutt your keyboard, whatever works for you.
    private final Random rng;


    public ExampleAgent(String host, Integer port, ProtocolManager.ConnectionType conType) {

        // TODO enter your data
        super(
                host,
                port,
                conType,
                "ExampleAgent",
                null, // authors go here
                null, // description goes here
                null, // token goes here
                true // don't print network communication
        );

        // Initialize your agent, load databases, neural networks, ...
        rng = new Random();
    }

    // You can also implement your own methods of course
    private static void sleep(long millis) {
        try {
            Thread.sleep(millis);
        } catch (InterruptedException e) {
            e.printStackTrace();
        }
    }

    @Override
    public void search(KalahState ks) throws IOException {

        // Pick a random move
        ArrayList<Integer> moves = ks.getMoves();
        int randomIndex = rng.nextInt(moves.size());
        int chosenMove = moves.get(randomIndex);

        // Send that move to the server, you can submit as many moves as you like, last one counts
        this.submitMove(chosenMove);

        // artificially wait until search time is over
        // takes 50 naps of 1 millisecond each
        int naps = 0;
        while (!shouldStop() && naps < 50) {
            sleep(1);
            naps++;
        }

        // The waiting is just for demonstration of should_stop(), you can return from search() at any time
        // Return from search() as soon as should_stop() returns true.
        // Check should_stop() at least 10 times a second, return quickly if it returns true,
        // there is nothing to be gained from submitting more moves, trust us
    }

    // Example of a main function
    public static void main(String[] args) throws IOException {

        while (true) {
            try {
                // Init agent
                Agent agent = new ExampleAgent("kalah.kwarc.info/socket", null, ProtocolManager.ConnectionType.WebSocketSecure);

                // Connect to the server and play games until something happens
                agent.run();
            } catch (IOException e) {
                e.printStackTrace();
            }
            // Wait 10 seconds before trying again
            sleep(10_000);
        }


        // For local tests (the server code is publicly available)
        // Agent agent = new ExampleAgent("localhost", 2671, ProtocolManager.ConnectionType.TCP);
    }

}
