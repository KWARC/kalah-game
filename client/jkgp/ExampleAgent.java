package kgp.info.kwarc.kalah.jkpg;

import java.io.IOException;
import java.util.ArrayList;
import java.util.Random;

// Simple example of an agent
// Chooses among the legal moves uniform at random, sends new "best" moves in increasing intervals
// the latter is just to make it a better example, we don't want to provide any algorithms here
public class ExampleAgent extends Agent {

    // Token could be any String, but PLEASE use a big non-empty(!!) randomized String for security's sake
    // and so we can distinguish your agents.
    // Concat together some random (decimal/hexadecimal) integers or headbutt your keyboard, whatever works for you.
    private static final String TOKEN = "10666affd0cde9c3c54b86ef6782d146bf055b8fc4a492ad46bb44d077df79ec3eac04c9a0c71a7ee5d4717f6348a1fd88d2a1819e21d32156c72000bedac5131476177c192a54fa08ae234c94ddb25d71b911b83ee610fe5541f630f73dbabd660c11abaa33534fbe51d11c32bae7f537a75bc19a4c3d85da77828b6f39f2f7";
    private final Random rng;


    public ExampleAgent(String host, Integer port, ProtocolManager.ConnectionType conType) {

        super(
                host,
                port,
                conType,
                "ExampleAgent",
                "Tobias Völk [Former Tutor]",
                "Totally sophisticated Kalah agent developed by Philip Kaludercic und Tobias Völk in 2021.\n\n" +
                        "Chooses among the legal moves uniform at random, pretends to be thinking.\n" +
                        "Very friendly to the environment.",
                TOKEN
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

    // Example of a main function
    public static void main(String[] args) throws IOException {

        // Prepare agent for playing on a server on, for example, the same machine
        // Agent initialization happens before we connect to the server
        // Note that tournament programs will start your client in a process and punish it
        // if it doesn't connect to the server within a specified amount of time

        // Use WebSocketSecure to connect to the training server
        // For WebSocket and WebSocketSecure you can enter a different port
        Agent agent = new ExampleAgent("kalah.kwarc.info/socket", null, ProtocolManager.ConnectionType.WebSocketSecure);

        // For local tests (the server code is publicly available) on the same PC use TCP,
        // the Kalah Game Protocol default port is 2671
        // Agent agent = new ExampleAgent("localhost", 2671, ProtocolManager.ConnectionType.TCP);

        // If necessary, do some other stuff here before connecting.
        // The game might start immediately after connecting!

        while (true) {
            try {
                // Connects to the server,
                // plays the tournament / game(s) until there's a fatal error or the server ends the connection
                agent.run();
            } catch (Exception e) {
                e.printStackTrace();
            }
            // Wait 10 seconds before trying again
            //sleep(10_000);
        }
    }

    @Override
    public void search(KalahState ks) throws IOException {

        // The actual "search". ShouldStop is checked in a loop but if you're doing a recursive search you might want
        // to check it every N nodes or every N milliseconds, just so it's called a few times per second,
        // as a good server punishes slow reactions to the stop command by subtracting the delay from the amount of
        // time for the next move

        int naps = 3;
        long timeToWait = 5; // 5 milliseconds
        while (!shouldStop() && naps > 0) {
            // Randomly decide whether to stop the search early
            // In practice you might stop when you know that your position is won - it speeds up the tournament!
            if (rng.nextDouble() < 0.1) {
                return;
            }

            // Pick a random move
            ArrayList<Integer> moves = ks.getMoves();
            int randomIndex = rng.nextInt(moves.size());
            int chosenMove = moves.get(randomIndex);

            // Send that move to the server
            this.submitMove(chosenMove);

            // Commenting on the current position and/or move choice
            sendComment("Currently best move: " + (chosenMove + 1) + "\n" +
                    "Evaluation: -3\n" +
                    "Computation steps: 5\n" +
                    "Emotion: \"happy\"");

            // Artificially sleeping, makes no sense in a real agent of course,
            sleep(timeToWait);
            naps--;
        }

        // This implementation doesn't return from search() until the server says so via shouldStop(),
        // but that would be perfectly fine, for example if your agent found a proven win
    }

}
