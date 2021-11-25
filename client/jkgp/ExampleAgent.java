package info.kwarc.kalah.jkpg;

import java.io.IOException;
import java.math.BigInteger;
import java.util.ArrayList;
import java.util.Random;

// simple example of an agent
// chooses among the legal moves uniform at random, sends new "best" moves in increasing intervals
// the latter is just to make it a better example, we don't want to provide any algorithms here
public class ExampleAgent extends Agent {

    // Token could be any String, but PLEASE use a big randomized String for security's sake
    // If you're lazy, just concat together some random (decimal/hexadecimal) integers
    private static final String TOKEN = "10666affd0cde9c3c54b86ef6782d146bf055b8fc4a492ad46bb44d077df79ec3eac04c9a0c71a7ee5d4717f6348a1fd88d2a1819e21d32156c72000bedac5131476177c192a54fa08ae234c94ddb25d71b911b83ee610fe5541f630f73dbabd660c11abaa33534fbe51d11c32bae7f537a75bc19a4c3d85da77828b6f39f2f7";
    private final Random rng;

    public ExampleAgent(String host, int port, boolean encrypted) {

        super(
                host,
                port,
                ProtocolManager.ConnectionType.WS,
                "ExampleAgentName",
                "Philip Kaludercic, Tobias Völk",
                "Sophisticated Kalah agent developed by Philip Kaludercic und Tobias Völk in 2021.\n\n" +
                        "Chooses among the legal moves uniform at random.\n" +
                        "Very friendly to the environment.",
                TOKEN
        );
        // Initialize your agent, load databases, neural networks, ...
        rng = new Random();
    }

    @Override
    public void search(KalahState ks) throws IOException {

        // Immediately send some legal move in case time runs out early
        submitMove(ks.lowestLegalMove());

        // The actual "search". ShouldStop is checked in a loop but if you're doing a recursive search you might want
        // to check it every N nodes or every N milliseconds, just so it's called a few times per second,
        // as a good server punishes slow reactions to the stop command by subtracting the delay from the amount of
        // time for the next move

        long timeToWait = 50;
        while (!shouldStop())
        {
            // randomly decide whether to stop search early
            // in practice you might stop when you know it's won etc. to speed up the tournament
            if (rng.nextDouble() < 0.1)
            {
                return;
            }

            // pick a random move
            ArrayList<Integer> moves = ks.getMoves();
            int randomIndex = rng.nextInt(moves.size());
            int chosenMove = moves.get(randomIndex);

            // send that move to the server
            this.submitMove(chosenMove);

            // Commenting on the current position and/or move choice
            sendComment("Currently best move: " + (chosenMove + 1) + "\n" +
                    "Evaluation: -3\n" +
                    "Computation steps: 5");

            sleep(timeToWait);

            timeToWait *= 2.0; // increase search time
        }

        // This implementation doesn't return from search() until the server says so,
        // but that would be perfectly fine, for example if your agent found a proven win
    }

    // you can also implement your own methods of course
    private static void sleep(long millis)
    {
        long start = System.currentTimeMillis();
        while(System.currentTimeMillis() < start + millis) {
            try {
                long remainingTime = (start + millis) - System.currentTimeMillis();
                Thread.sleep(remainingTime);
            } catch (InterruptedException e) {
                e.printStackTrace();
            }
        }
    }

    // Example of a main function
    public static void main(String[] args) throws IOException {

        // Prepare agent for playing on a server on, for example, the same machine
        // Agent initialization happens before we connect to the server
        // Not that tournament programs might start your client in a process and punish it
        // if it doesn't connect to the server within a specified amount of time
        // 2671/2672 is the Kalah Game Protocol default port for unencrypted/encrypted
        Agent agent = new ExampleAgent("localhost", 2671, false); // TODO adapt encrypted default port

        // If necessary, do some other stuff here before connecting.
        // The game might start immediately after connecting!

        // Connects to the server, plays the tournament / game, ends the connection. Handles everything.
        agent.run();
    }

}
