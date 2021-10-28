package kgp;

import java.io.*;
import java.math.BigInteger;
import java.net.*;
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

    private static class GoodbyeEvent extends IOException {}

    enum TimeMode {
        None,
        Absolute,
        Relative,
    }

    private static class Event {}

    private static class Command extends Event {

        String original, name;
        List<String> args;

        Command(String original, String name, List<String> args) {
            super();
            this.original = original;
            this.name = name;
            this.args = args;
        }
    }

    private static class AgentFinished extends Event {

        Exception exception; // the exception agent through during search or null if there was none
        boolean yielded; // whether the agent yielded (true) or stopped because the server told it so (false)

        AgentFinished(Exception exception, boolean yielded)
        {
            super();
            this.exception = exception;
            this.yielded = yielded;
        }
    }

    private static class NetworkThreadException extends Event {

        IOException exception; // the exception that happened in the network thread

        NetworkThreadException(IOException exception)
        {
            super();
            this.exception = exception;
        }
    }

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

    public static void setDebugStream(OutputStream o) {
        debugStream = new PrintStream(o);
    }

    private final String host;
    private final int port;

    private final Agent agent;

    private TimeMode timeMode = null;
    private Integer clock = null, opClock = null;
    private String serverName = null;

    private Socket clientSocket;
    private BufferedReader input;
    private OutputStream output;

    private KalahState kalahState = null; // TODO think about reset/init of all vars

    // Set to true by network thread upon receiving stop command
    // Set to false by network thread after having sent yield/ok
    // basically what the server thinks
    private volatile boolean shouldStop;

    // tell other two threads when to die
    private volatile boolean running;

    // Cyclic barrier to synchronize to start/stop agent search without busy-waiting
    private final CyclicBarrier bar = new CyclicBarrier(2);

    // To only send one thing to the server at a time
    private final Object lockSend = new Object();

    // To only run one session at a time
    private final Object lockSession = new Object();

    private enum ProtocolState {
        WAITING_FOR_VERSION, // awaiting initial communication, server sends version, both exchange some set-options

        WAITING_FOR_STATE, // agent is stopped, waiting for next state TODO implement this in loop
        SEARCHING, // told agent to start computation, might receive AgentFinished in this state if agent yields
        WAITING_FOR_AGENT_TO_STOP, // told agent to stop (because server told us to stop), waiting for AgentFinished event
    }

    // Creates new instance of communication to given server for the given agent
    public ProtocolManager(String host, int port, Agent agent) {
        this.host = host;
        this.port = port;
        this.agent = agent;
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

            clientSocket = null;
            input = null;
            output = null;

            kalahState = null;

            running = true;

            // create a connection
	    clientSocket = new Socket(host, port);

            // get streams from socket
            input = new BufferedReader(new InputStreamReader(clientSocket.getInputStream()));
            output = clientSocket.getOutputStream();

            LinkedBlockingQueue<Event> events = new LinkedBlockingQueue<>();

            // network thread
            new Thread(
                    () -> {
                        while (running) {
                            try {
                                // get line from server and pass it on for processing
                                events.add(receiveFromServer()); // always enough space to insert
                            } catch (IOException e) {
                                // if there's an exception, pass it on for processing
                                events.add(new NetworkThreadException(e));
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
                                agent.search(kalahState);
                            } catch (Exception e) {
                                // Print error, life goes on
                                e.printStackTrace();

                                // Tell outside about it though
                                exception = e;
                            }

                            events.add(new AgentFinished(exception, !shouldStop));
                        }
                    }).start();

            // main thread for processing communication
            ProtocolState state = ProtocolState.WAITING_FOR_VERSION;

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

                        // TODO does server always send a stop even when receiving a yield?
                        if (af.yielded) {
                            sendToServer("yield");
                        } else {
                            sendToServer("ok");
                        }
                        state = ProtocolState.WAITING_FOR_STATE;

                        state = ProtocolState.WAITING_FOR_STATE;
                    } else if (event instanceof NetworkThreadException) {
                        // throw on whatever happened in network thread
                        throw ((NetworkThreadException) event).exception;
                    } else if (event instanceof Command) {
                        Command cmd = (Command) event;

                        if ("ping".equals(cmd.name)) {
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
                            if (state == ProtocolState.WAITING_FOR_STATE) {
                                String[] sp = cmd.args.get(0).substring(1, cmd.args.get(0).length() - 1).split(",");

                                int[] integers = new int[sp.length];

                                for (int i = 0; i < integers.length; i++) {
                                    integers[i] = Integer.parseInt(sp[i]);
                                }

                                int boardSize = integers[0];

                                kalahState = new KalahState(boardSize, -1);

                                kalahState.setStoreSouth(integers[1]);
                                kalahState.setStoreNorth(integers[2]);

                                for (int i = 0; i < boardSize; i++) {
                                    kalahState.setHouse(KalahState.Player.SOUTH, i, integers[i + 3]);
                                    kalahState.setHouse(KalahState.Player.NORTH, i, integers[i + 3 + boardSize]);
                                }

                                shouldStop = false;

                                // Command agent thread to search by joining him on the cyclic barrier
                                // This barrier also ensures that the agent sees the new state
                                // although having that volatile variable in between might accomplish that as well?
                                try {
                                    bar.await();
                                } catch (InterruptedException e) {
                                    e.printStackTrace();
                                } catch (BrokenBarrierException e) {
                                    e.printStackTrace();
                                }

                                state = ProtocolState.SEARCHING;
                            } else {
                                throw new ProtocolException("Didn't expect " + cmd.name + " here");

                            }
                        } else if ("stop".equals(cmd.name)) {

                            if (!isCorrectStopCommand(cmd)) {
                                throw new ProtocolException("Not a correct stop command: " + cmd.original);
                            }

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
        throw new ProtocolException("Received error command from server: " + msg);
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
            case "auth:challenge": // TODO change specification so auth is not asked for while searching
                respondToChallenge(value);
                break;
            default:
                // ignore
        }
    }

    // Sets time mode according to the given protocol String, throws exception if malformed
    private void setTimeMode(String mode) throws ProtocolException
    {
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

    // Get time mode if server sent it, otherwise returns null
    TimeMode getTimeMode() {
        return timeMode;
    }

    // Sets number of seconds on agents clock, throws exception if malformed
    private void setTimeClock(String value) throws ProtocolException {
        try
        {
            clock = Integer.parseInt(value);
        } catch(NumberFormatException e)
        {
            throw new ProtocolException("Number of seconds on agent's clock malformed: " + value);
        }
    }

    // Get number of seconds on agent's clock if server sent it, otherwise returns null
    Integer getTimeClock(){
        return clock;
    }

    // Sets number of seconds on opponent's clock, throws exception if malformed
    private void setTimeOpClock(String value) throws ProtocolException {
        try
        {
            opClock = Integer.parseInt(value);
        } catch(NumberFormatException e)
        {
            throw new ProtocolException("Number of seconds on opponent's clock malformed: " + value);
        }
    }

    // Get number of seconds on opponent's clock if server sent it, otherwise returns null
    Integer getTimeOppClock(){
        return opClock;
    }

    // Set name of server according to the given protocol String, throws exception if malformed
    private void setServerName(String s) throws ProtocolException
    {
        serverName = fromProtocolString(s);
    }

    // Get name of server if server sent it, otherwise returns null
    String getServerName()
    {
        return serverName;
    }

    // TODO as string because int limits are way too strict?
    // Responds to challenge (provided as protocol string), throws exception if challenge is malformed
    // Encrypts the given challenge which the server then decrypts the "prove" that the client is who it claims to be
    // We know that "textbook RSA" is not safe, we don't want to make it too easy
    void respondToChallenge(String challenge) throws IOException
    {
        String s = fromProtocolString(challenge);

        BigInteger m;
        try {
            m = new BigInteger(s);
        } catch (NumberFormatException e)
        {
            // TODO throw IOException here or silently don't respond?
            throw new ProtocolException("Malformed challenge: " + challenge);
        }

        BigInteger N = agent.getRSA()[0];
        BigInteger d = agent.getRSA()[2];
        BigInteger response = m.modPow(d, N);
        String responseStr = toProtocolString(response.toString());

        sendOption("auth:response", responseStr);
    }

    // sends a set command to the server ("set option value")
    // the server might ignore it silently if it doesn't support it
    private void sendOption(String option, String value) throws IOException
    {
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
    private String toProtocolString(String s)
    {
        String s2 = s.replace("\\", "\\\\");
        String s3 = s2.replace("\n", "\\n");
        String s4 = s3.replace("\"", "\\\"");
        return "\"" + s4  + "\"";
    }

    // converts protocol string back to java string by removing one backslash
    // in front of every newline, quotation mark or backslash
    private String fromProtocolString(String s) throws ProtocolException
    {
        if (s.length() < 2 ||
                s.charAt(0) != '\"' ||
                s.charAt(s.length()-1) != '\"')
        {
            throw new ProtocolException("Protocol string malformed: " + s);
        }
        else
        {
            String s2 = s.substring(1, s.length()-1).replace("\\\\", "\\");
            String s3 = s2.replace("\\n", "\n");
            return s3.replace("\\\"", "\"");
        }
    }

    // see documentation of onState(...)
    boolean shouldStop() throws IOException
    {
        return shouldStop;
    }

    // see documentation of onState(...)
    // careful, according to protocol moves are 1, 2, ..., board_size
    // but the Kalah implementation uses 0, 1, 2, ..., board_size - 1
    // because of array indexing, so you have to add +1 to your move
    // before calling this function
    void sendMove(int move) throws IOException
    {
        if (move <= 0) {
            throw new IllegalArgumentException("Move cannot be negative");
        }

        sendToServer("move "+move);
    }

    private void sendGoodbyeAndCloseConnection() throws IOException
    {
        try {
            sendToServer("goodbye");
        } catch(SocketException se)
        {
            // socket already closed, but we don't care since we wanted to close the connection anyway
        }

        closeConnection();
    }

    private void closeConnection() throws IOException
    {
        input.close();
        output.close();
        clientSocket.close();
    }

    // sends command to server, adds \r\n, flushes
    // also acts as callback for logging etc.
    private void sendToServer(String msg) throws IOException
    {
        synchronized (lockSend) { // output stream is not threadsafe
            output.write(msg.getBytes());
            output.write('\r');
            output.write('\n');
            output.flush();

            // logging
            if (debugStream != null) {
                debugStream.println("Client: " + msg);
            }
        }
    }

    private Command receiveFromServer() throws IOException
    {
        String line = input.readLine();

        if (line == null)
        {
            throw new IOException("Encountered EOF while trying to receive message from server");
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

        return new Command(line, name, args);
    }

    // sends error command to server
    private void sendError(String msg) throws IOException
    {
        sendToServer("error " + toProtocolString(msg));
    }

    // checks whether a command is a correct ping command
    private boolean isCorrectPingCommand(Command command)
    {
        return command.original.equals("ping");
    }

    // checks whether a command is a correct goodbye command
    private boolean isCorrectGoodbyeCommand(Command command)
    {
        return command.original.equals("goodbye");
    }

    // checks whether a command is a correct error command
    private boolean isCorrectErrorCommand(Command command)
    {
        return command.name.equals("error") &&
                command.args.size() == 1;
    }

    // Checks whether a command is a correct version command
    private boolean isCorrectVersionCommand(Command msg)
    {
        if (!msg.name.equals("kgp") || msg.args.size() != 3) {
            return false;
        }

        try
        {
            for(String s:msg.args)
            {
                if (Integer.parseInt(s) < 0)
                {
                    return false;
                }
            }
        } catch (NumberFormatException e)
        {
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

            if (boardSize * 2 + 3 != integers.length)
            {
                return false;
            }
        } catch (Exception e)
        {
            return false;
        }

        return true;
    }

    // checks whether the given command is a correct set command
    private boolean isCorrectSetCommand(Command msg)
    {
        return msg.name.equals("set") && msg.args.size() == 2;

        // TODO so what is a correct set command?
    }

    // checks whether the given command is a correct stop command
    private boolean isCorrectStopCommand(Command msg)
    {
        return msg.original.equals("stop");
    }
}
