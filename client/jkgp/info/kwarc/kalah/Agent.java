package info.kwarc.kalah;

import java.io.IOException;

public abstract class Agent {

    private final ProtocolManager com;
    private String name, authors, description, token;
    private ProtocolManager.ConnectionType conType;

    /**
     * For TCP the port has to be non-null, for WebSocket(Secure) it'll use the default ports
     * If name, authors, description, token are null, then that value will not be sent to the server upon connecting
     *
     * @param host Server to connect to
     * @param port Port to connect via, can be null
     * @param conType Type of connection TCP, WebSocket, WebSocketSecure
     * @param name Name of the agent, can be null
     * @param authors Authors of the agent, can be null
     * @param description Description of the agent, can be null
     * @param token Token of the agent, can be null
     * @param printNetwork Whether to print client-server-communication to stdout
     */
    public Agent(String host, Integer port, ProtocolManager.ConnectionType conType, String name, String authors, String description, String token, boolean printNetwork) {

        com = new ProtocolManager(host, port, conType, this, printNetwork);

        this.name = name;
        this.authors = authors;
        this.description = description;

        this.token = token;

        this.conType = conType;
    }


    /**
     * Called when the server sends a state to the client e.g. asks it to start computing moves for that state.
     * Important: Also see should_stop()
     * @param ks A board with south to move
     * @throws IOException If something goes wrong during the computation of the move
     */
    public abstract void search(KalahState ks) throws IOException;


    /**
     * @return Agent's name, null if not specified.
     */
    public String getName() {
        return name;
    }

    /**
     * @return Agent's authors, null if not specified.
     */
    public String getAuthors() {
        return authors;
    }

    /**
     * @return Agent's description, null if not specified.
     */
    public String getDescription() {
        return description;
    }

    /**
     * @return Agent's token or null if not specified.
     */
    public String getToken() {
        return token;
    }

    /**
     * Connects to the server, plays games until the server closes the connection or an error occurs.
     * Exits the connection in a clean way upon error and passes the Exception to the caller as an IOException.
     */
    public final void run() throws IOException {
        com.run();
    }

    /**
     * Tells the server the move the agent currently wants to play.
     * Use during search() only.
     */
    protected final void submitMove(int move) throws IOException {
        com.sendMove(move + 1);
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
        return com.shouldStop();
    }

    /**
     * Sends a comment about the current position to the server,
     * Use during search() only.
     * Comment may include linebreaks and newlines
     */
    protected final void sendComment(String comment) throws IOException {
        com.sendComment(comment);
    }

    /**
     * @return Time mode if set, otherwise null
     */
    protected final ProtocolManager.TimeMode getTimeMode() {
        return com.getTimeMode();
    }

    /**
     * @return Number of seconds on agent's clock, otherwise null
     */
    protected final Integer getTimeClock() {
        return com.getTimeClock();
    }

    /**
     * @return Number of seconds on opponent's clock, otherwise null
     */
    protected final Integer getTimeOppClock() {
        return com.getTimeOppClock();
    }

    /**
     * @return Server name if available, otherwise null
     */
    protected final String getServerName() {
        return com.getServerName();
    }
}