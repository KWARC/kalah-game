package info.kwarc.kalah;

import java.io.IOException;
import java.io.PrintStream;
import java.net.ProtocolException;
import java.net.SocketException;
import java.sql.Timestamp;
import java.util.*;
import java.util.concurrent.CyclicBarrier;
import java.util.concurrent.LinkedBlockingQueue;
import java.util.concurrent.ConcurrentHashMap;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

// This protocol implementation of the Kalah Game Protocol 1.0.0 is kinda "robust"
// (= little mercy regarding protocol errors + handles errors in a clean way)
// Shuts down cleanly in case of Exceptions (client crashes), passes the Exception on to the caller
// Notifies the server of the server's protocol errors (wrong command / at the wrong time), errors and agent errors

/**
 * Internal protocol handling logic.
 */
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
    private PrintStream debugStream;
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
    private ProtocolState state = null;
    // Set to true by network thread upon receiving stop command
    // Set to false by network thread after having sent yield/ok
    // basically what the server thinks
    // tell other two threads when to die
    private volatile boolean running;
    private ConnectionType conType;

    private ConcurrentHashMap<Long, Integer> runningStateRefs;
    private LinkedBlockingQueue<Job> jobs;

    private final Job POISON_PILL_JOB = new Job(null, null);
    
    private long id;

    class Job {
        KalahState ks;
        Long id;

        Job(KalahState ks, Long id) {
            this.ks = ks;
            this.id = id;
        }
    }

    // Creates new instance of communication to given server for the given agent
    ProtocolManager(String host, Integer port, ConnectionType conType, Agent agent, boolean printNetwork) {

        if (printNetwork) {
            debugStream = System.out;
        } else {
            debugStream = null;
        }

        if (port != null && conType != ConnectionType.TCP) {
            throw new IllegalArgumentException(
                    "Don't set port when using WebSocket or WebSocketSecure. Set port=null for the official server," +
                            " otherwise add port into hostname");
        }
        this.host = host;
        this.port = port;
        this.conType = conType;
        this.agent = agent;
    }

    // Connects to the server, handles the tournament/game/..., then ends the connection
    // Don't use run in parallel, create more ProtocolManager instances instead
    // This method will block successive calls until the previous session is over
    void run() throws IOException {

        synchronized (lockSession) {
            // reset data for this session
            timeMode = null;
            clock = null;
            opClock = null;
            serverName = null;

            runningStateRefs = new ConcurrentHashMap<>();
            jobs = new LinkedBlockingQueue<>();

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

	    id = 1L;

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
                            Long id = null;
                            try {
                                Job job = jobs.take();
                                if (job == POISON_PILL_JOB) {
                                    return;
                                }
                                id = job.id;
                                agent.do_search(job.ks, job.id);
                                events.add(new AgentFinished(null, id));
                            } catch (IOException e) {
                                events.add(new AgentFinished(e, id));
                            } catch (InterruptedException e) {
                            	e.printStackTrace();
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
                        e.printStackTrace();
                    }

                    if (event instanceof AgentFinished) { // agent stopped/yielded
                        AgentFinished af = (AgentFinished) event;
                        
                        if (af.exception != null) {
                        	throw af.exception;
                        }

                        if (!shouldStop(af.ref)) {
                            sendYield(af.ref);
                            runningStateRefs.remove(id);
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
                                    sendToServer("mode freeplay", null);

                                    state = ProtocolState.MAIN;
                                }
                            } else {
                                throw new ProtocolException("Didn't expect " + cmd.name + " here");
                            }
                        } else if ("state".equals(cmd.name)) {
                            if (!isCorrectStateCommand(cmd)) {
                                throw new ProtocolException("Not a correct state command: " + cmd.original);
                            }

                            String[] sp = cmd.args.get(0).substring(1, cmd.args.get(0).length() - 1).split(",");

                            int[] integers = new int[sp.length];

                            for (int i = 0; i < integers.length; i++) {
                                integers[i] = Integer.parseInt(sp[i]);
                            }

                            int boardSize = integers[0];

                            KalahState ks = new KalahState(boardSize, -1);

                            ks.setStoreSouth(integers[1]);
                            ks.setStoreNorth(integers[2]);

                            for (int i = 0; i < boardSize; i++) {
                                ks.setHouse(KalahState.Player.SOUTH, i, integers[i + 3]);
                                ks.setHouse(KalahState.Player.NORTH, i, integers[i + 3 + boardSize]);
                            }

                            if (ks.getHouseSumSouth() == 0) {
                                // no legal moves
                                throw new ProtocolException("Server sent state with no legal moves:\n" + ks);
                            }

                            jobs.add(new Job(ks, cmd.id));
                            runningStateRefs.put(cmd.id, Integer.MAX_VALUE); // Hopefully, Integer.MAX_VALUE is not a legal move

                        } else if ("stop".equals(cmd.name)) {
                            runningStateRefs.remove(cmd.ref);
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

                // insert poison pill to stop agent thread
                jobs.add(POISON_PILL_JOB);

                // agent thread (if agent isn't broken) escapes via fake stop:
                runningStateRefs.clear();

                // or it's hung up in the barrier
                bar.reset();

                // on crash, don't tell the server, just say goodbye and still throw the exception
                sendGoodbyeAndCloseConnection();
            }
        }
    }

    private void sendYield(Long ref) throws IOException {
        assert ref != null;
        sendToServer("yield", ref);
    }

    // react to ping
    private void reactToPing(Command cmd) throws IOException {
        sendToServer("pong", cmd.id);
    }

    // react to error
    private void reactToError(Command msg) throws ProtocolException {
        if (!isCorrectErrorCommand(msg)) {
            throw new ProtocolException("Not a correct error command: " + msg.original);
        }

        System.err.println("Received error from server: " + msg.original);
        debugStream.println("Received error from server: " + msg.original);
        // throw new ProtocolException("Received error command from server: " + msg.original);
    }

    // react to goodbye
    private void reactToGoodbye(Command cmd) throws GoodbyeEvent, ProtocolException {
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
        sendToServer("set " + option + " " + value, null);
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
    boolean shouldStop(Long ref) {
        return !runningStateRefs.containsKey(ref);
    }

    // see documentation of onState(...)
    // careful, according to protocol moves are 1, 2, ..., board_size
    // but the Kalah implementation uses 0, 1, 2, ..., board_size - 1
    // because of array indexing, so you have to add +1 to your move
    // before calling this function
    synchronized void sendMove(int move, Long ref) throws IOException {
    	assert ref != null;
        
        if (move <= 0) {
            throw new IllegalArgumentException("Move cannot be negative");
        }

        Integer lastMove = runningStateRefs.get(ref);
        if (lastMove != null && !lastMove.equals(move)) {
            sendToServer("move " + move, ref);
            runningStateRefs.put(ref, move);
        }
    }

    private void sendGoodbyeAndCloseConnection() throws IOException {
        try {
            sendToServer("goodbye", null);
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
    private synchronized void sendToServer(String msg, Long ref) throws IOException {
    	String msg2 = id + (ref == null ? "" : "@" + ref) + " " + msg;
    
    	connection.send(msg2);
    	
    	id += 2;
    	if (id < 0) { // Overflow
    	    throw new ProtocolException("Client side ID overflowed Long (64bit signed integer)");
    	}

        // logging
        if (debugStream != null) {
            Timestamp t = new Timestamp(System.currentTimeMillis());
            debugStream.println("["+t+"] Client: " + msg2);
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
            Timestamp t = new Timestamp(System.currentTimeMillis());
            debugStream.println("["+t+"] Server: " + line);
        }

        Matcher mat = commandPattern.matcher(line);
        if (!mat.matches()) {
            throw new ProtocolException("Malformed input: " + line);
        }


	Long id;
	if (mat.group(1) == null) {
	    id = null;
	} else {
	    id = idStringToLong(mat.group(1));
	}
	
        Long ref;
        if (mat.group(2) == null) {
	    ref = null;
	} else {
	    ref = idStringToLong(mat.group(2));
	}
        
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
                id,
                ref,
                name,
                args);
    }

    private static Long idStringToLong(String ref) throws IOException {
        assert ref != null;
        
        Long r = null;
        
        try {
            r = Long.parseLong(ref);
        } catch (NumberFormatException e) {
            throw new IOException("Unable to parse ref to Long (signed 64 bit integer): " + ref);
        }
        
        return r;
    }

    // sends error command to server
    private void sendError(String msg, Long ref) throws IOException {
        sendToServer("error " + toProtocolString(msg), ref);
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
        MAIN, // Not waiting for version anymore
    }

    /** Enum for the three possible connection types. */
    public enum ConnectionType {
        /**
         * TCP
         */
        TCP,

        /**
         * Websocket
         */
        WebSocket,

        /**
         * WebSocketSecure
         */
        WebSocketSecure,
    }

    private static class GoodbyeEvent extends IOException {
    }

    private static class Event {
    }

    private static class Command extends Event {

        String original, name;
        Long id, ref;
        List<String> args;

        Command(String original, Long id, Long ref, String name, List<String> args) {
            super();
            this.original = original;
            this.id = id;
            this.ref = ref;
            this.name = name;
            this.args = args;
        }

        @Override
        public String toString() {
            return this.original;
        }
    }

    private static class AgentFinished extends Event {

        IOException exception; // the exception agent through during search or null if there was none
        Long ref; // ref of the state for which the computation finished

        AgentFinished(IOException exception, Long ref) {
            super();
            this.exception = exception;
            this.ref = ref;
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
