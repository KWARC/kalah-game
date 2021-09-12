package kgp;

import java.io.IOException;
import java.util.ArrayList;
import java.util.Random;

// simple example of an agent
// chooses among the legal moves uniform at random, sends new "best" moves in increasing intervals
// the latter is just to make it a better example, we don't want to provide any algorithms here
public class ExampleAgent extends Agent {

    private final Random rng;

    public ExampleAgent(String host, int port) {
        super(host, port);

        // Initialize your agent, load databases, neural networks, ...
        rng = new Random();
    }

    @Override
    public String getName()
    {
        return "ExampleAgentName";
    }

    @Override
    public String getAuthors()
    {
        return "Philip Kaludercic, Tobias Völk";
    }

    @Override
    public String getDescription() {
        return "Sophisticated Kalah agent developed by Philip Kaludercic und Tobias Völk in 2021.\n\n" +
                "Chooses among the legal moves uniform at random.\n" +
                "Very friendly to the environment.";
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
            // pick a random move
            ArrayList<Integer> moves = ks.getMoves();
            int randomIndex = rng.nextInt(moves.size());
            int chosenMove = moves.get(randomIndex);

            // send that move to the server
            this.submitMove(chosenMove);

            // Commenting on the current position and/or move choice
            sendComment("I chose move " + (chosenMove + 1) + " because the RNG told me to so.\n" +
                    "evaluation: " + "How am I supposed to know??\n" +
                    "\"You shouldn't have played that move, you're DOOOMED!");

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
        // 2671 is the Kalah Game Protocol default port
        Agent agent = new ExampleAgent("localhost", 2671);

        // If necessary, do some other stuff here before connecting.
        // The game might start immediately after connecting!

        // Connects to the server, plays the tournament / game, ends the connection. Handles everything.
        agent.run();
    }

}