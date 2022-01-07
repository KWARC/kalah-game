package info.kwarc.kalah;

import java.io.IOException;
import java.util.concurrent.atomic.AtomicBoolean;

/**
 * Superclass to extend agents from.
 */
public abstract class Agent {

    private ProtocolManager com;
    private final String name, authors, description, token;
    private String id;
    private KalahState ks;
    private final AtomicBoolean running;

    /**
     * Creates an agent which is ready to connect to the specified server, introducing itself with the given attributes
     * if provided. See parameters for details.
     *
     * @param host Server to connect to.
     * @param port Port to connect via, should be null iff you're using WebSocket or WebSocketSecure.
     * @param conType Type of connection TCP, WebSocket, WebSocketSecure.
     * @param name Name of the agent, can be null.
     * @param authors Authors of the agent, can be null.
     * @param description Description of the agent, can be null.
     * @param token Token of the agent, can be null.
     * @param printNetwork Whether to print client-server-communication to stdout.
     */
    public Agent(String host, Integer port, ProtocolManager.ConnectionType conType, String name, String authors, String description, String token, boolean printNetwork) {

        com = new ProtocolManager(host, port, conType, this, printNetwork);

        this.name = name;
        this.authors = authors;
        this.description = description;
        this.token = token;

        this.running = new AtomicBoolean(false);
    }

    // This is called to hide the setting of id and state and to silently ensure the sequential execution
    final void do_search(KalahState ks, String id) throws IOException {

        // Already running?
        boolean isRunning = running.getAndSet(true);
        if (isRunning) {
            throw new IOException("Attempt to call Agent.search() in parallel, use multiple Agent instances instead");
        }

        assert this.id == null;
        assert this.ks == null;

        // To ensure that the agent is sending the right id for the right states
        // Note that this cannot be called in parallel

        this.id = id;
        this.ks = new KalahState(ks);
        this.search(ks);
        this.id = null;
        this.ks = null;

        // Is this thread-safe? I think and hope so
        running.set(false);
    }

    /**
     * Called when the server sends a state to the client e.g. asks it to start computing moves for that state.
     * Important: Also see should_stop()
     * @param ks A board with south to move
     * @throws IOException If something goes wrong during the computation of the move
     */
    public abstract void search(KalahState ks) throws IOException;

    /** Returns agent's name or null if not specified. */
    public String getName() {
        return name;
    }

    /** Returns agent's authors or null if not specified. */
    public String getAuthors() {
        return authors;
    }

    /** Returns agent's description or null if not specified. */
    public String getDescription() {
        return description;
    }

    /** Returns agent's token or null if not specified. */
    public String getToken() {
        return token;
    }

    /**
     * Connects to the server, plays games until the server closes the connection or an error occurs.
     * Exits the connection in a clean way upon error and passes the Exception to the caller as an IOException.
     * @throws IOException In case an I/O error occurs (also includes protocol and agent errors).
     */
    public final void run() throws IOException {
        com.run();
    }

    /**
     * Tells the server the move the agent currently wants to play.
     * Use during search() only.
     * @param move Move to submit. Moves are indexed from 0 to N-1 in sowing direction.
     * @throws IOException If something goes wrong with I/O
     */
    protected final void submitMove(int move) throws IOException {
        assert this.id != null;
        assert this.ks != null;
        assert this.ks.isLegalMove(move);

        if (!this.ks.isLegalMove(move)) {
            throw new IOException("Agent tried to send illegal move " + (move+1) + " on this board:\n"+ks);
        }
        com.sendMove(move + 1, this.id);
    }

    /**
     * Call this function at least 10 times per second in search()
     * and end the search as fast as possible when true is returned.
     * There is nothing to be gained from a delayed reaction, in fact
     * this behaviour is usually punished.
     * Note that you can still return from search() at any time, even
     * when should_stop() doesn't return true yet.
     * @return True after the server told the client to stop searching
     */
    protected final boolean shouldStop() {
        return com.shouldStop(this.id);
    }

    /**
     * Sends a comment about the current position to the server,
     * Use during search() only.
     * @param comment The comment to send. May include line breaks.
     * @throws IOException If something goes wrong with I/O.
     */
    protected final void sendComment(String comment) throws IOException {
        com.sendComment(comment);
    }

    /** Returns time mode if available or null otherwise */
    protected final ProtocolManager.TimeMode getTimeMode() {
        return com.getTimeMode();
    }

    /** Returns number of seconds on agents clock if available or null otherwise */
    protected final Integer getTimeClock() {
        return com.getTimeClock();
    }

    /** Returns number of seconds on opponent's clock if available or null otherwise */
    protected final Integer getTimeOppClock() {
        return com.getTimeOppClock();
    }

    /** Returns server name if available or null otherwise */
    protected final String getServerName() {
        return com.getServerName();
    }
}