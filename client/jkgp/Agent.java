package kgp;

import java.io.IOException;

// TODO maybe remove/replace sendOption etc. as the library should hide the protocol
// though that would mean that every server would have to develop it's own library

// Implement the constructor and search() for your agent according to the comments, and you're done
// Not knowing the board size upon creation of the agent is on purpose
// Note that servers might punish agents whose constructor needs too much time
// Prints the client-server communication to the error stream
// Btw. this is a single threaded implementation, the agent and the protocol manager are calling each other
// So don't worry about scheduling issues if your tournament restricts the agent to one CPU core
public abstract class Agent {

    // Set common option values to be sent automatically upon initialization
    // or return null if you don't want to send that option
    protected abstract String getName();
    protected abstract String getAuthors();
    protected abstract String getDescription();

    // Called after the connection has been established
    // Can be used to exchange custom set commands with the server
    protected abstract void beforeGameStarts() throws IOException;

    // MANDATORY: Read the documentation of submitMove() and shouldStop()
    // Find the best move for the given state here
    // It's always south's turn to move
    protected abstract void search(KalahState ks) throws IOException;

    // the communication instance the agent uses to communicate with the server
    private final ProtocolManager com;

    // Creates an agent to play with a local server using the Kalah Game Protocol default port 2671
    public Agent() {
        com = new ProtocolManager("localhost", 2671, this);
    }

    // Creates an agent to play with the specified server using the specified port
    public Agent(String host, int port) {
        com = new ProtocolManager(host, port, this);
    }

    // connects to the server, plays the tournament/game/whatever, closes the connection
    // passes on IOExceptions
    // if there is a mistake in the implementation and the client crashes, it will notify the server and end
    // the connection before crashing the program
    public final void run() throws IOException {
        com.run();
    }

    // TODO etc. move set comment into submitMove, optional comment through overloading?

    // Tell the server the currently "best" move (according to your agent)
    // Call at least one time or the server might punish you!
    // A move is a number in [0 ..., board_size-1] (in the direction of sowing)
    protected final void submitMove(int move) throws IOException {
        com.sendMove(move + 1);
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

    // Tells the server to comment on the current position, call it during search() for example
    // Pass your string, can include linebreaks but no quotation marks
    // Throws IOException if the comment does contain quotation marks
    protected final void sendComment(String comment) throws IOException {
        com.sendComment(comment);
    }

    // Call this function to send an option with its value to the server
    // Check the specification for when to send what option
    protected final void sendOption(String option, String value) throws IOException {
        com.sendOption(option, value);
    }

    // Get time mode from server if it was sent, otherwise returns null
    protected final Integer getTimeMode(){
        String mode = com.getServerOptionValue("time:mode");
        if (mode == null)
        {
            return null;
        }
        else
        {
            return Integer.parseInt(mode);
        }
    }

    // Get number of seconds on your clock
    protected final Integer getClock(){
        String clock = com.getServerOptionValue("time:clock");
        if (clock == null)
        {
            return null;
        }
        else
        {
            return Integer.parseInt(clock);
        }
    }

    // Get number of seconds on your clock
    protected final Integer getOppClock(){
        String clock = com.getServerOptionValue("time:opclock");
        if (clock == null)
        {
            return null;
        }
        else
        {
            return Integer.parseInt(clock);
        }
    }

    // Returns the value of an option if the server has sent it
    // Otherwise returns null
    // Check the specification for common examples
    protected final String getOption(String option)
    {
        return com.getServerOptionValue(option);
    }

}