package kgp;

import java.io.IOException;

// Implement init() and search() for your agent according to the comments, and you're done
// Prints the client-server communication to the error stream
// Btw. this is a single threaded implementation, the agent and the protocol manager are calling each other
// So don't worry about scheduling issues if your tournament restricts the agents to one CPU core
public abstract class Agent {

    // Initialize your agent here like loading databases, neural network files, ...
    // Servers might throw an error if it takes too long
    // Don't bother doing expensive calculations elsewhere:
    //  -you don't know the size of the board yet
    //  -even if you do, a good server will subtract that time from your initialization time
    //  -you might as well do your calculations on your gaming pc at home and hardcode the results = opening book
    protected abstract void init(int boardSize);

    // MANDATORY: Read the documentation of submitMove() and shouldStop()
    // Find the best move for the given state here
    // It's always south's turn to move
    protected abstract void search(KalahState ks) throws IOException;






    // the communication instance the agent uses to communicate with the server
    private final ProtocolManager com;

    // Creates an agent to play with a local server using the Kalah Game Protocol default port 2671
    public Agent() throws IOException{
        com = new ProtocolManager("localhost", 2671, this);
    }

    // Creates an agent to play with the specified server using the specified port
    public Agent(String host, int port) throws IOException {
        com = new ProtocolManager(host, port, this);
    }

    // connects to the server, plays the tournament/game/whatever, closes the connection
    // passes on IOExceptions
    // if there is a mistake in the implementation and the client crashes, it will notify the server and end
    // the connection before crashing the program
    public final void run() throws IOException {
        com.run();
    }

    // Tell the server the currently "best" move (according to your agent)
    // Call at least one time or the server might punish you!
    // A move is a number in [1, ..., board_size] (in the direction of sowing)
    protected final void submitMove(int move)
    {
        com.sendMove(move);
    }

    // Call this function a few times per second in search()
    // Calling this function regularly is necessary to handle the protocol communication!
    // Returns true after the server told the client to stop searching.
    // End search as fast as possible if it returns true! A good server implementation
    // subtracts overtime from the next move to prevent cheating!
    // Note: You can return from search() anytime, for example if you don't need more time
    protected final boolean shouldStop() throws IOException {
        return com.shouldStop();
    }

}