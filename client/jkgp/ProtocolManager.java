package kgp;

import java.io.*;
import java.net.*;
import java.util.LinkedList;
import java.util.List;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

// This protocol implementation of the Kalah Game Protocol 1.0.0 is kinda "robust"
// (= little mercy regarding protocol errors + handles errors in a clean way)
// Shuts down cleanly in case of Exceptions (client crashes), passes the Exception on to the caller
// Notifies the server of the server's protocol errors (wrong message / at the wrong time), errors and agent errors

public class ProtocolManager {

    private class GoodbyeEvent extends IOException {}

    enum TimeMode {
        None,
        Absolute,
        Relative,
    }

    // TODO rename message to command in all comments, when you have too much time
    private static class Command {

        String original, name;
        List<String> args;

        public Command(String original, String name, List<String> args) {
            this.original = original;
            this.name = name;
            this.args = args;
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

    private final String host;
    private final int port;

    private final Agent agent;

    private TimeMode timeMode = null;
    private Integer clock = null, opClock = null;
    private String serverName = null;

    private Socket clientSocket;
    private BufferedReader input;
    private OutputStream output;

    private boolean serverSaidStop;

    private enum ProtocolState {
        WAITING_FOR_VERSION,
        PLAYING,
    }

    // Creates new instance of communication to given server for the given agent
    public ProtocolManager(String host, int port, Agent agent) {
        this.host = host;
        this.port = port;
        this.agent = agent;
    }

    // Connects to the server, handles the tournament/game/..., then ends the connection
    public void run() throws IOException {
        // create a connection
        clientSocket = new Socket(host, port);
        input = new BufferedReader(new InputStreamReader(clientSocket.getInputStream()));
        output = clientSocket.getOutputStream();

        ProtocolState state = ProtocolState.WAITING_FOR_VERSION;

        try {
            label:
            while (true) {
                Command msg = receiveFromServer();

                if ("ping".equals(msg.name)) {
                    if (isIncorrectPingMessage(msg)) {
                        throw new ProtocolException("Not a correct ping message: " + msg.original);
                    }
                    sendToServer("pong");
                } else if ("error".equals(msg.name)) {
                    if (isIncorrectErrorMessage(msg)) {
                        throw new ProtocolException("Not a correct error message: " + msg.original);
                    }
                    throw new ProtocolException("Received error message from server: " + msg);
                } else if ("goodbye".equals(msg.name)) {
                    if (isIncorrectGoodbyeMessage(msg)) {
                        throw new ProtocolException("Not a correct goodbye message: " + msg.original);
                    }
                    throw new GoodbyeEvent();
                } else if ("kgp".equals(msg.name)) {
                    if (!isCorrectVersionMessage(msg)) {
                        throw new ProtocolException("Not a correct version message: " + msg.original);
                    }
                    if (state == ProtocolState.WAITING_FOR_VERSION) {
                        if (!msg.original.startsWith("kgp 1 ")) {
                            // wrong protocol version
                            throw new ProtocolException("Only kgp 1.*.* supported");
                        } else {
                            // server uses kalah game protocol 1.*.*

                            // send the default information
                            if (agent.getName() != null)
                                sendOption("info:name", agent.getName());
                            if (agent.getAuthors() != null)
                                sendOption("info:authors", agent.getAuthors());
                            if (agent.getDescription() != null)
                                sendOption("info:description", agent.getDescription());

                            // supported version, reply with mode
                            sendToServer("mode simple");

                            // and wait for the initialization message
                            state = ProtocolState.PLAYING;
                        }
                    } else {
                        throw new ProtocolException("Didn't expect " + msg.name + " here");
                    }
                } else if ("state".equals(msg.name)) {
                    if (!isCorrectStateMessage(msg)) {
                        throw new ProtocolException("Not a correct state message: " + msg.original);
                    }
                    if (state == ProtocolState.PLAYING) {
                        String[] sp = msg.args.get(0).substring(1, msg.args.get(0).length() - 1).split(",");

                        int[] integers = new int[sp.length];

                        for (int i = 0; i < integers.length; i++) {
                            integers[i] = Integer.parseInt(sp[i]);
                        }

                        int boardSize = integers[0];

                        KalahState ks = new KalahState(boardSize, -1);

                        ks.setStoreSouth(integers[1]);
                        ks.setStoreNorth(integers[2]);

                        for (int i = 0; i < boardSize; i++) {
                            ks.setHouse(KalahState.Side.SOUTH, i, integers[i + 3]);
                            ks.setHouse(KalahState.Side.NORTH, i, integers[i + 3 + boardSize]);
                        }

                        // search, can take a long time
                        onState(ks);
                    } else {
                        throw new ProtocolException("Didn't expect " + msg.name + " here");

                    }
                } else if ("set".equals(msg.name)) {
                    if (!isCorrectSetMessage(msg)) {
                        throw new ProtocolException("Not a correct set message: " + msg.original);
                    }
                    String option = msg.args.get(0);
                    String value = msg.args.get(1);

                    switch (option) {
                        case "info:name":
                            setServerName(value);
                            break label;
                        case "time:mode":
                            setTimeMode(value);
                            break label;
                        case "time:clock":
                            setTimeClock(value);
                            break label;
                        case "time:opclock":
                            setTimeOpClock(value);
                            break label;
                    }
                } else {
                    throw new ProtocolException("Unknown command " + msg.name);
                }
            }
        } catch(GoodbyeEvent ge)
        {
            // do nothing, just don't crash
        }
        finally {
            // on crash, don't tell the server, just say goodbye and still throw the exception
            sendGoodbyeAndCloseConnection();
        }
    }

    // Sets time mode, throws exception if malformed
    private void setTimeMode(String mode) throws ProtocolException
    {
        switch (mode) {
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

    // Set name of server, throws exception if malformed
    private void setServerName(String s) throws ProtocolException
    {
        serverName = fromProtocolString(s);
    }

    // Get name of server if server sent it, otherwise returns null
    String getServerName()
    {
        return serverName;
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
            throw new ProtocolException("Server name malformed: " + s);
        }
        else
        {
            String s2 = s.substring(1, s.length()-1).replace("\\\\", "\\");
            String s3 = s2.replace("\\n", "\n");
            String s4 = s3.replace("\\\"", "\"");
            return s4;
        }
    }

    // called when the server tells the client to start searching
    private void onState(KalahState ks) throws IOException, GoodbyeEvent
    {
        serverSaidStop = false;

        try {
            agent.search(ks);
        }
        catch(Exception e)
        {
            sendMove(ks.randomLegalMove());
            sendComment("[jkgp] Agent crashed (" + e.getClass().getSimpleName() + "), move chosen randomly.");
        }

        if (serverSaidStop)
        {
            // server told us to stop
            // tell server that we're done stopping
            sendToServer("ok");
        }
        else
        {
            // agent decided to stop
            sendToServer("yield");

            // wait for it's stop
            while (true) {
                Command msg = receiveFromServer();

                if ("stop".equals(msg.name)) {
                    if (isIncorrectStopMessage(msg)) {
                        throw new IOException("Not a correct stop message: " + msg.original);
                    }

                    // server told us to stop

                    // agent already stopped (yielded), do nothing
                    return;
                } else if ("ping".equals(msg.name)) {
                    if (isIncorrectPingMessage(msg)) {
                        throw new ProtocolException("Not a correct ping message: " + msg.original);
                    }

                    sendToServer("pong");
                } else if ("error".equals(msg.name)) {
                    if (isIncorrectErrorMessage(msg)) {
                        throw new ProtocolException("Not a correct error message: " + msg.original);
                    }

                    throw new IOException("Server error: " + msg.original);
                } else if ("goodbye".equals(msg.name)) {
                    if (isIncorrectGoodbyeMessage(msg)) {
                        throw new ProtocolException("Not a correct goodbye message: " + msg.original);
                    }

                    throw new GoodbyeEvent();
                } else {
                    throw new ProtocolException("Got " + msg.name +
                            " but expected ping/error/goodbye/stop while waiting for stop");
                }
            }
        }
    }

    // see documentation of onState(...)
    boolean shouldStop() throws IOException, GoodbyeEvent
    {
        if (serverSaidStop)
        {
            return true;
        }
        else if (input.ready())
        {
            Command msg = receiveFromServer();

            if ("stop".equals(msg.name)) {
                if (isIncorrectStopMessage(msg)) {
                    throw new ProtocolException("Not a correct stop message: " + msg.original);
                }

                // set stop variable for subsequent calls
                serverSaidStop = true;

                // tell agent to stop
                return true;
            } else if ("ping".equals(msg.name)) {
                if (isIncorrectPingMessage(msg)) {
                    throw new ProtocolException("Not a correct ping message: " + msg.original);
                }

                sendToServer("pong");
            } else if ("error".equals(msg.name)) {
                if (isIncorrectErrorMessage(msg)) {
                    throw new ProtocolException("Not a correct error message: " + msg.original);
                }

                throw new IOException("Server error: " + msg.original);
            } else if ("goodbye".equals(msg.name)) {
                if (isIncorrectGoodbyeMessage(msg)) {
                    throw new ProtocolException("Not a correct goodbye message: " + msg.original);
                }

                throw new GoodbyeEvent();
            } else {
                throw new ProtocolException("Got " + msg.name +
                        " but expected ping/error/goodbye/stop while checking for stop");
            }
        }

        // tell agent to continue
        return false;
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

    // sends message to server, adds \r\n, flushes
    // also acts as callback for logging etc.
    private void sendToServer(String msg) throws IOException
    {
        output.write(msg.getBytes());
        output.write('\r');
        output.write('\n');
        output.flush();

        System.err.println("Client: " + msg);
    }

    private Command receiveFromServer() throws IOException
    {
        String line = input.readLine();

        // logging
        System.err.println("Server: " + line);

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

    // sends error message to server
    private void sendError(String msg) throws IOException
    {
        sendToServer("error " + toProtocolString(msg));
    }

    // checks whether a message is a correct ping message
    private boolean isIncorrectPingMessage(Command command)
    {
        return !command.original.equals("ping");
    }

    // checks whether a message is a correct goodbye message
    private boolean isIncorrectGoodbyeMessage(Command command)
    {
        return !command.original.equals("goodbye");
    }

    // checks whether a message is a correct error message
    private boolean isIncorrectErrorMessage(Command command)
    {
        return !command.name.equals("error") ||
                command.args.size() != 1;
    }

    // Checks whether a message is a correct version message
    private boolean isCorrectVersionMessage(Command msg)
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

    // checks whether a message is a correct state message
    private boolean isCorrectStateMessage(Command msg) {

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

    // checks whether the given message is a correct set message
    private boolean isCorrectSetMessage(Command msg)
    {
        if (!msg.name.equals("set") && msg.args.size() != 2)
        {
            return false;
        }

        return !msg.args.get(0).contains(":");
    }

    // checks whether the given message is a correct stop message
    private boolean isIncorrectStopMessage(Command msg)
    {
        return !msg.original.equals("stop");
    }
}
