package kgp.info.kwarc.kalah.jkpg;

import java.io.*;
import java.net.ProtocolException;
import java.net.SocketException;
import java.util.LinkedList;
import java.util.List;
import java.util.concurrent.BrokenBarrierException;
import java.util.concurrent.CyclicBarrier;
import java.util.concurrent.LinkedBlockingQueue;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

// This protocol implementation of the Kalah Game Protocol 1.0.0 is kinda "robust"
// (= little mercy regarding protocol errors + handles errors in a clean way)
// Shuts down cleanly in case of Exceptions (client crashes), passes the Exception on to the caller
// Notifies the server of the server's protocol errors (wrong command / at the wrong time), errors and agent errors

public class ProtocolManager {

    private static final Pattern commandPattern = Pattern.compile(
            "^\\s*(?:(\\d+)?(?:@(\\d+))?\\s+)?" + // id and reference
                    "([a-z0-9]+)\\s*" + // command name
                    "((?:\\s+(?:" +
                    "[-+]?\\d+|" + // integer values
                    "[-+]?\\d*\\.\\d+?|" + // real values
                    "[a-z0-9:-]+|" + // words
                    "\"(?:\\\\.|[^\"])*\"|" + // strings
                    "<\\d+(,\\d+)*>" + // board
                    "))*\\s*)$",
            Pattern.CASE_INSENSITIVE);
    private static final Pattern argumentPattern = Pattern.compile(
            "[-+]?\\d+|" + // integer values
                    "[-+]?\\d*\\.\\d+?|" + // real values
                    "[a-z0-9:-]+|" + // words
                    "\"(?:\\\\.|[^\"])*\"|" + // strings
                    "<\\d+(,\\d+)*>",
            Pattern.CASE_INSENSITIVE);
    private static PrintStream debugStream = System.out;
    private final String host;
    private final Integer port;
    private final Agent agent;
    // Cyclic barrier to synchronize to start/stop agent search without busy-waiting
    private final CyclicBarrier bar = new CyclicBarrier(2);
    // To only run one session at a time
    private final Object lockSession = new Object();
    private TimeMode timeMode = null;
    private Integer clock = null, opClock = null;
    private String serverName = null;
    private Connection connection;
    private KalahState kalahStateManager = null;
    private KalahState kalahStateAgent = null;
    private ProtocolState state = null;
    // Set to true by network thread upon receiving stop command
    // Set to false by network thread after having sent yield/ok
    // basically what the server thinks
    private volatile boolean shouldStop;
    // tell other two threads when to die
    private volatile boolean running;
    private ConnectionType conType;
    // Creates new instance of communication to given server for the given agent
    public ProtocolManager(String host, Integer port, ConnectionType conType, Agent agent) {

        if (port == null) {
            this.host = host;
        } else {
            this.host = host + ":" + port;
        }
        this.port = port;
        this.conType = conType;
        this.agent = agent;
    }

    public static void setDebugStream(OutputStream o) {
        debugStream = new PrintStream(o);
    }

    // Connects to the server, handles the tournament/game/..., then ends the connection
    // Don't use run in parallel, create more ProtocolManager instances instead
    // This method will block successive calls until the previous session is over
    public void run() throws IOException {

        synchronized (lockSession) {
            // reset data for this session
            timeMode = null;
            clock = null;
            opClock = null;
            serverName = null;

            kalahStateManager = null;
            kalahStateAgent = null;

            state = ProtocolState.WAITING_FOR_VERSION;

            running = true;

            // create a connection
            if (conType == ConnectionType.TCP) {
                connection = new ConnectionTCP(host, port);
            } else if (conType == ConnectionType.WebSocket) {
                connection = new ConnectionWebSocket("ws://" + host);
            } else {
                connection = new ConnectionWebSocket("wss://" + host);
            }

            LinkedBlockingQueue<Event> events = new LinkedBlockingQueue<>();

            // network thread
            new Thread(
                    () -> {
                        while (running) {
                            try {
                                // get line from server and pass it on for processing
                                Command msg = receiveFromServer();
                                events.add(msg); // always enough space to insert
                            } catch (IOException e) {
                                // if there's an exception, pass it on for processing
                                events.add(new NetworkThreadException(e));
                                break;
                            }
                        }
                    }).start();

            // agent thread
            new Thread(
                    () -> {
                        while (running) {
                            // Agent thread is waiting for network thread to join, to start playing
                            // Also acts as synchronization to make sure this thread sees the current KalahState
                            try {
                                bar.await();
                            } catch (InterruptedException e) {
                                // not supposed to happen, just die
                                e.printStackTrace();
                                System.exit(1);
                            } catch (BrokenBarrierException e) {
                                return; // we were told to die or something wasn't supposed to happen
                            }

                            Exception exception = null;

                            try {
                                // send copy so you can change the other one without
                                // having to wait for the agent to finish
                                agent.search(kalahStateAgent);
                            } catch (Exception e) {
                                // Print error, life goes on
                                e.printStackTrace();

                                // Tell outside about it though
                                exception = e;
                                break;
                            } finally {
                                events.add(new AgentFinished(exception));
                            }
                        }
                    }).start();


            // main thread for processing communication

            try {
                while (true) {
                    Event event = null;

                    try {
                        event = events.take();
                    } catch (InterruptedException e) {
                        // not supposed to happen, just die
                        e.printStackTrace();
                        System.exit(1);
                    }

                    if (event instanceof AgentFinished) { // agent stopped/yielded
                        AgentFinished af = (AgentFinished) event;

                        sendToServer("yield");

                        if (kalahStateManager == null) {
                            // Did not receive state command yet, but agent is ready now
                            state = ProtocolState.WAITING_FOR_STATE;
                        } else {
                            startGame();
                        }

                    } else if (event instanceof NetworkThreadException) {
                        // throw on whatever happened in network thread
                        throw ((NetworkThreadException) event).exception;
                    } else if (event instanceof Command) {
                        Command cmd = (Command) event;

                        if ("ok".equals(cmd.name)) {
                            // do nothing
                        } else if ("ping".equals(cmd.name)) {
                            reactToPing(cmd);
                        } else if ("error".equals(cmd.name)) {
                            reactToError(cmd);
                        } else if ("goodbye".equals(cmd.name)) {
                            reactToGoodbye(cmd);
                        } else if ("kgp".equals(cmd.name)) {
                            if (!isCorrectVersionCommand(cmd)) {
                                throw new ProtocolException("Not a correct version command: " + cmd.original);
                            }
                            if (state == ProtocolState.WAITING_FOR_VERSION) {
                                if (!cmd.original.startsWith("kgp 1 ")) {
                                    // wrong protocol version
                                    throw new ProtocolException("Only kgp 1.*.* supported");
                                } else {
                                    // server uses kalah game protocol 1.*.*

                                    // send the default information
                                    if (agent.getName() != null)
                                        sendOption("info:name", toProtocolString(agent.getName()));
                                    if (agent.getAuthors() != null)
                                        sendOption("info:authors", toProtocolString(agent.getAuthors()));
                                    if (agent.getDescription() != null)
                                        sendOption("info:description", toProtocolString(agent.getDescription()));
                                    if (agent.getToken() != null) {
                                        sendOption("auth:token", toProtocolString(agent.getToken()));
                                    }

                                    // supported version, reply with mode
                                    sendToServer("mode simple");

                                    // and wait for the first state command
                                    state = ProtocolState.WAITING_FOR_STATE;
                                }
                            } else {
                                throw new ProtocolException("Didn't expect " + cmd.name + " here");
                            }
                        } else if ("state".equals(cmd.name)) {
                            if (!isCorrectStateCommand(cmd)) {
                                throw new ProtocolException("Not a correct state command: " + cmd.original);
                            }
                            if (state == ProtocolState.WAITING_FOR_STATE ||
                                    state == ProtocolState.WAITING_FOR_AGENT_TO_STOP) {

                                // Just set state

                                String[] sp = cmd.args.get(0).substring(1, cmd.args.get(0).length() - 1).split(",");

                                int[] integers = new int[sp.length];

                                for (int i = 0; i < integers.length; i++) {
                                    integers[i] = Integer.parseInt(sp[i]);
                                }

                                int boardSize = integers[0];

                                kalahStateManager = new KalahState(boardSize, -1);

                                kalahStateManager.setStoreSouth(integers[1]);
                                kalahStateManager.setStoreNorth(integers[2]);

                                for (int i = 0; i < boardSize; i++) {
                                    kalahStateManager.setHouse(KalahState.Player.SOUTH, i, integers[i + 3]);
                                    kalahStateManager.setHouse(KalahState.Player.NORTH, i, integers[i + 3 + boardSize]);
                                }

                                if (kalahStateManager.getHouseSumSouth() == 0) {
                                    // no legal moves
                                    throw new ProtocolException("Server sent state with no legal moves:\n" + kalahStateManager);
                                }

                                // Only start searching if the agent is ready

                                if (state == ProtocolState.WAITING_FOR_STATE) {
                                    startGame();
                                } else if (state == ProtocolState.WAITING_FOR_AGENT_TO_STOP) {
                                    // waiting for agent to stop
                                } else {
                                    throw new RuntimeException("Wrong internal state");
                                }

                            } else {
                                sendError("ABC");
                                throw new ProtocolException("Didn't expect " + cmd.name + " here");

                            }
                        } else if ("stop".equals(cmd.name)) {

                            // Don't care whether stop has any arguments (it's not supposed to have any)

                            if (state == ProtocolState.SEARCHING) {

                                // Tell agent thread to stop
                                shouldStop = true;

                                // Then go about other business
                                // The agent will notify this loop via an event

                                state = ProtocolState.WAITING_FOR_AGENT_TO_STOP;
                            } else if (state == ProtocolState.WAITING_FOR_STATE) {
                                // just ignore it, might be a stop which was sent before the server received a yield
                            } else {
                                throw new ProtocolException("Didn't expect " + cmd.name + " here");
                            }
                        } else if ("set".equals(cmd.name)) {
                            reactToSet(cmd);
                        } else {
                            throw new ProtocolException("Unknown command " + cmd.name);
                        }
                    }
                }
            } catch (GoodbyeEvent ge) {
                // do nothing, just don't crash
            } catch (IOException ie) {
                // print stack trace, but don't send error to server, just say goodbye
                ie.printStackTrace();
            } finally {
                // tell the threads to stop their while loops
                running = false;

                // network thread escapes from readLine() via IOError

                // agent thread (if agent isn't broken) escapes via fake stop:
                shouldStop = true;

                // or it's hung up in the barrier
                bar.reset();

                // on crash, don't tell the server, just say goodbye and still throw the exception
                sendGoodbyeAndCloseConnection();
            }
        }
    }

    // start game
    private void startGame() {
        // state and agent are available, start searching!
        shouldStop = false;

        // copy state to agent so you can change it without disturbing the agent
        kalahStateAgent = new KalahState(kalahStateManager);
        kalahStateManager = null;

        // Command agent thread to search by joining him on the cyclic barrier
        // This barrier also ensures that the agent sees the new state
        // although having that volatile variable in between might accomplish that as well?
        try {
            bar.await();
        } catch (InterruptedException | BrokenBarrierException e) {
            e.printStackTrace();
        }

        state = ProtocolState.SEARCHING;
    }

    // react to ping
    private void reactToPing(Command cmd) throws IOException {
        if (!isCorrectPingCommand(cmd)) {
            throw new ProtocolException("Not a correct ping command: " + cmd.original);
        }
        sendToServer("pong");
    }

    // react to error
    private void reactToError(Command msg) throws ProtocolException {
        if (!isCorrectErrorCommand(msg)) {
            throw new ProtocolException("Not a correct error command: " + msg.original);
        }

        System.err.println("Received and ignored error command from server: " + msg.original);
        // throw new ProtocolException("Received error command from server: " + msg.original);
    }

    // react to goodbye
    private void reactToGoodbye(Command cmd) throws GoodbyeEvent, ProtocolException {
        if (!isCorrectGoodbyeCommand(cmd)) {
            throw new ProtocolException("Not a correct goodbye command: " + cmd.original);
        }
        throw new GoodbyeEvent();
    }

    // react to set
    private void reactToSet(Command cmd) throws IOException {
        if (!isCorrectSetCommand(cmd)) {
            throw new ProtocolException("Not a correct set command: " + cmd.original);
        }
        String option = cmd.args.get(0);
        String value = cmd.args.get(1);

        switch (option) {
            case "info:name":
                setServerName(value);
                break;
            case "time:mode":
                setTimeMode(value);
                break;
            case "time:clock":
                setTimeClock(value);
                break;
            case "time:opclock":
                setTimeOpClock(value);
                break;
            default:
                // ignore
        }
    }

    // Get time mode if server sent it, otherwise returns null
    TimeMode getTimeMode() {
        return timeMode;
    }

    // Sets time mode according to the given protocol String, throws exception if malformed
    private void setTimeMode(String mode) throws ProtocolException {
        switch (fromProtocolString(mode)) {
            case "none":
                timeMode = TimeMode.None;
                break;
            case "absolute":
                timeMode = TimeMode.Absolute;
                break;
            case "relative":
                timeMode = TimeMode.Relative;
                break;
            default:
                throw new ProtocolException("Unknown time mode " + mode);
        }
    }

    // Get number of seconds on agent's clock if server sent it, otherwise returns null
    Integer getTimeClock() {
        return clock;
    }

    // Sets number of seconds on agents clock, throws exception if malformed
    private void setTimeClock(String value) throws ProtocolException {
        try {
            clock = Integer.parseInt(value);
        } catch (NumberFormatException e) {
            throw new ProtocolException("Number of seconds on agent's clock malformed: " + value);
        }
    }

    // Sets number of seconds on opponent's clock, throws exception if malformed
    private void setTimeOpClock(String value) throws ProtocolException {
        try {
            opClock = Integer.parseInt(value);
        } catch (NumberFormatException e) {
            throw new ProtocolException("Number of seconds on opponent's clock malformed: " + value);
        }
    }

    // Get number of seconds on opponent's clock if server sent it, otherwise returns null
    Integer getTimeOppClock() {
        return opClock;
    }

    // Get name of server if server sent it, otherwise returns null
    String getServerName() {
        return serverName;
    }

    // Set name of server according to the given protocol String, throws exception if malformed
    private void setServerName(String s) throws ProtocolException {
        serverName = fromProtocolString(s);
    }

    // sends a set command to the server ("set option value")
    // the server might ignore it silently if it doesn't support it
    private void sendOption(String option, String value) throws IOException {
        // no need to check, only library is using this function
        // so option and value have to be correct
        sendToServer("set " + option + " " + value);
    }

    // send comment to server, check comment for quotation marks
    void sendComment(String comment) throws IOException {
        // place a backslash in front of every backslash, newline and quotation mark
        sendOption("info:comment", toProtocolString(comment));
    }

    // converts java string to protocol string by replacing backslash, newline and quotation marks
    // with their backslash-lead counterparts
    private String toProtocolString(String s) {
        String s2 = s.replace("\\", "\\\\");
        String s3 = s2.replace("\n", "\\n");
        String s4 = s3.replace("\"", "\\\"");
        return "\"" + s4 + "\"";
    }

    // converts protocol string back to java string by removing one backslash
    // in front of every newline, quotation mark or backslash
    private String fromProtocolString(String s) throws ProtocolException {
        if (s.length() < 2 ||
                s.charAt(0) != '\"' ||
                s.charAt(s.length() - 1) != '\"') {
            throw new ProtocolException("Protocol string malformed: " + s);
        } else {
            String s2 = s.substring(1, s.length() - 1).replace("\\\\", "\\");
            String s3 = s2.replace("\\n", "\n");
            return s3.replace("\\\"", "\"");
        }
    }

    // see documentation of onState(...)
    boolean shouldStop() {
        return shouldStop;
    }

    // see documentation of onState(...)
    // careful, according to protocol moves are 1, 2, ..., board_size
    // but the Kalah implementation uses 0, 1, 2, ..., board_size - 1
    // because of array indexing, so you have to add +1 to your move
    // before calling this function
    void sendMove(int move) throws IOException {
        if (move <= 0) {
            throw new IllegalArgumentException("Move cannot be negative");
        }

        if (kalahStateAgent.getHouse(KalahState.Player.SOUTH, move - 1) == 0) {
            throw new IllegalArgumentException("Agent tried to send illegal move " + move + ":\n" + kalahStateAgent);
        }

        // TODO REMOVE CHECK

        sendToServer("move " + move);
    }

    private void sendGoodbyeAndCloseConnection() throws IOException {
        try {
            sendToServer("goodbye");
        } catch (SocketException se) {
            // socket already closed, but we don't care since we wanted to close the connection anyway
        }

        closeConnection();
    }

    private void closeConnection() throws IOException {
        connection.close();
    }

    // sends command to server, adds \r\n, flushes
    // also acts as callback for logging etc.
    private void sendToServer(String msg) throws IOException {
        connection.send(msg);

        // logging
        if (debugStream != null) {
            debugStream.println("Client: " + msg);
        }
    }

    private Command receiveFromServer() throws IOException {
        String line;
        try {
            line = connection.receive();
        } catch (InterruptedException ie) {
            throw new IOException("Interrupted Exception: " + ie.getMessage());
        }

        if (line == null) {
            throw new IOException("Server closed connection without saying goodbye, client sad :(");
        }

        // logging
        if (debugStream != null) {
            debugStream.println("Server: " + line);
        }

        Matcher mat = commandPattern.matcher(line);
        if (!mat.matches()) {
            throw new ProtocolException("Malformed input: " + line);
        }

        // ignore id and ref for this implementation
        String name = mat.group(3);
        List<String> args = new LinkedList<>();

        int i = 0;
        Matcher arg = argumentPattern.matcher(mat.group(4));
        while (arg.find(i)) {
            args.add(arg.group());
            i = arg.end();
        }

        return new Command(line.substring(
                mat.start(3)),
                name,
                args);
    }

    // sends error command to server
    private void sendError(String msg) throws IOException {
        sendToServer("error " + toProtocolString(msg));
    }

    // checks whether a command is a correct ping command
    private boolean isCorrectPingCommand(Command command) {
        return command.original.equals("ping");
    }

    // checks whether a command is a correct goodbye command
    private boolean isCorrectGoodbyeCommand(Command command) {
        return command.original.equals("goodbye");
    }

    // checks whether a command is a correct error command
    private boolean isCorrectErrorCommand(Command command) {
        return command.name.equals("error") &&
                command.args.size() == 1;
    }

    // Checks whether a command is a correct version command
    private boolean isCorrectVersionCommand(Command msg) {
        if (!msg.name.equals("kgp") || msg.args.size() != 3) {
            return false;
        }

        try {
            for (String s : msg.args) {
                if (Integer.parseInt(s) < 0) {
                    return false;
                }
            }
        } catch (NumberFormatException e) {
            return false;
        }

        return true;
    }

    // checks whether a command is a correct state command
    private boolean isCorrectStateCommand(Command msg) {

        if (!msg.name.equals("state") || msg.args.size() != 1) {
            return false;
        }

        try {
            String state = msg.args.get(0);

            String[] sp = state.substring(1, state.length() - 1).split(",");

            int[] integers = new int[sp.length];

            for (int i = 0; i < integers.length; i++) {
                integers[i] = Integer.parseInt(sp[i]);
            }

            int boardSize = integers[0];

            if (boardSize * 2 + 3 != integers.length) {
                return false;
            }
        } catch (Exception e) {
            return false;
        }

        return true;
    }

    // checks whether the given command is a correct set command
    private boolean isCorrectSetCommand(Command msg) {
        return msg.name.equals("set") && msg.args.size() == 2;
    }

    enum TimeMode {
        None,
        Absolute,
        Relative,
    }

    private enum ProtocolState {
        WAITING_FOR_VERSION, // awaiting initial communication, server sends version, both exchange some set-options

        WAITING_FOR_STATE, // agent is stopped, waiting for next state
        SEARCHING, // told agent to start computation, might receive AgentFinished in this state if agent yields
        WAITING_FOR_AGENT_TO_STOP, // told agent to stop (because server told us to stop), waiting for AgentFinished event
    }

    public enum ConnectionType {
        TCP,
        WebSocket,
        WebSocketSecure,
    }

    private static class GoodbyeEvent extends IOException {
    }

    private static class Event {
    }

    private static class Command extends Event {

        String original, name;
        List<String> args;

        Command(String original, String name, List<String> args) {
            super();
            this.original = original;
            this.name = name;
            this.args = args;
        }

        @Override
        public String toString() {
            return this.original;
        }
    }

    private static class AgentFinished extends Event {

        Exception exception; // the exception agent through during search or null if there was none

        AgentFinished(Exception exception) {
            super();
            this.exception = exception;
        }
    }

    private static class NetworkThreadException extends Event {

        IOException exception; // the exception that happened in the network thread

        NetworkThreadException(IOException exception) {
            super();
            this.exception = exception;
        }
    }

}
