package kgp;

import java.io.IOException;
import java.util.ArrayList;
import java.util.Random;

// simple example of an agent
// chooses among the legal moves uniform at random, sends new "best" moves in increasing intervals
// the latter is just to make it a better example, we don't want to provide any algorithms here
public class ExampleAgent extends Agent {

    private Random rng = null;

    public ExampleAgent(String host, int port) throws IOException {
        super(host, port);
    }

    @Override
    public void init(int boardSize)
    {
        rng = new Random();
    }

    @Override
    public void search(KalahState ks) throws IOException {

        // Immediately send some legal move in case time runs out early
        submitMove(ks.lowestLegalMove());

        long timeToWait = 50; // initially wait 50 ms

        // The actual "search". ShouldStop is checked in a loop but if you're doing a recursive search you might want
        // to check it every N nodes, just so it's called a few times per second, as a good server punishes slow
        // reactions to the stop command by subtracting the delay from the amount of time for the next move
        while (!shouldStop())
        {
            ArrayList<Integer> moves = ks.getMoves();
            int randomIndex = rng.nextInt(moves.size());
            int chosenMove = moves.get(randomIndex);

            this.submitMove(chosenMove + 1);

            try
            {
                Thread.sleep(timeToWait);
            }
            catch(InterruptedException e)
            {

            }

            timeToWait *= 2.0; // increase search time
        }
    }

    // Example of a main function
    public static void main(String[] args) throws IOException {

        // Prepare agent for playing on a server on, for example, the same machine
        // 2671 is the Kalah Game Protocol default port
        Agent agent = new ExampleAgent("localhost", 2671);

        // Connects to the server, plays the tournament / game, ends the connection. Handles everything.
        agent.run();
    }

}