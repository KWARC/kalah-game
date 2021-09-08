package kgp;

import java.io.*;
import java.net.*;
import java.util.Arrays;
import java.util.HashMap;
import java.util.LinkedList;
import java.util.List;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

// This protocol implementation is kinda robust
// Its only weakness is the acceptance of any remaining message after a correct state message
// Shuts down cleanly in case of Exceptions (client crashes), passes on the Exception
// Notifies the server of the server's protocol errors (wrong message / at the wrong time),
// agent crashes and other exceptions

public class ProtocolManager {

    private class Command {
        int id, ref;
        String name;
        List<String> args;

        public Command(int id, int ref, String name, List<String> args) {
            this.id = id;
            this.ref = ref;
            this.name = name;
            this.args = args;
        }
    }

    static private Pattern compat = Pattern.compile(
            "^\\s*(?:(\\d*)(?:@(\\d+)\\s+))?" + // id and reference
                    "([a-z0-9]+)\\s*" + // command name
                    "((?:\\s+(?:" +
                    "[-+]?\\d+|" + // integer values
                    "[-+]?\\d*\\.\\d+?|" + // real values
                    "[a-z0-9:-]+|" + // words
                    "\"(?:\\\\.|[^\"])*\"|" + // strings
                    "<\\d+(,\\d)*>" + // board
                    "))*\\s*)$",
            Pattern.CASE_INSENSITIVE);

    static private Pattern argpat = Pattern.compile(
                    "[-+]?\\d+|" + // integer values
                    "[-+]?\\d*\\.\\d+?|" + // real values
                    "[a-z0-9:-]+|" + // words
                    "\"(?:\\\\.|[^\"])*\"|" + // strings
                    "<\\d+(,\\d)*>",
            Pattern.CASE_INSENSITIVE);

    private final String host;
    private final int port;

    private final Agent agent;

    // For storing the values of options sent by set commands
    private final HashMap<String, String> serverOptions = new HashMap<>();

    private Socket clientSocket;
    private BufferedReader input;
    private PrintStream output;

    private boolean serverSaidStop;

    private enum ProtocolState
    {
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
    public void run() throws IOException
    {
        // create a connection
        clientSocket = new Socket(host, port);
        input = new BufferedReader(new InputStreamReader(clientSocket.getInputStream()));
        output = new PrintStream(clientSocket.getOutputStream(), true);

        ProtocolState state = ProtocolState.WAITING_FOR_VERSION;

        try {
            while (true) {
                String msg = receiveFromServer();

                if (msg.equals("ping")) {
                    sendToServer("pong");
                } else if (isErrorMessage(msg)) {
                    throw new IOException("Received error message from server: " + msg);
                } else if (msg.equals("goodbye")) {
                    throw new IOException("Server said goodbye");
                } else if (isVersionMessage(msg)) {
                    if (state == ProtocolState.WAITING_FOR_VERSION) {
                        if (!msg.startsWith("kgp 1 ")) {
                            // wrong protocol version
                            throw new IOException("Only kgp 1.*.* supported");
                        } else {
                            // supported version, reply with mode
                            sendToServer("mode simple");

                            // also send the default set information
                            sendOption("info:name", agent.getName());
                            sendOption("info:authors", agent.getAuthors());
                            sendOption("info:description", agent.getDescription());

                            // and wait for initialization message
                            state = ProtocolState.PLAYING;
                        }
                    }
                    else
                    {
                        throw new IOException("Didn't expect " + msg + " here");
                    }
                } else if (isStateMessage(msg)) {
                    if (state == ProtocolState.PLAYING) {
                        String[] sp = msg.substring(7, msg.length() - 1).split(",");
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
                    }
                    else if(isSetMessage(msg))
                    {
                        String[] sp = msg.split(" ");
                        String option = sp[1];
                        String value = sp[2];

                        serverOptions.put(option, value);
                    }
                    else
                    {
                        throw new IOException("Didn't expect " + msg +" here");
                    }
                }
                else
                {
                    throw new IOException("Server sent unknown or (slightly wrong?) message " + msg);
                }
            }
        }
        catch(IOException e)
        {
            sendError(e.getMessage().replaceAll("\n", "\\n"));
            throw e;
        }
        finally
        {
            sendGoodbyeAndCloseConnection();
        }
    }

    // returns the value of the option as String if the server ever sent
    // a set command with that option ("set option value"), otherwise returns null
    String getServerOptionValue(String option)
    {
        return serverOptions.get(option);
    }

    // sends a set message to the server ("set option value")
    // the server might ignore it silently if it doesn't support it
    void sendOption(String option, String value)
    {
        sendToServer("set " + option + " " + value);
    }

    // send comment to server, check comment for quotation marks
    void sendComment(String comment) throws IOException
    {
        if (comment.contains("\""))
        {
            throw new IOException("Comment may not contain quotation mark \"");
        }
        sendOption("info:comment", "\"" + comment + "\"");
    }

    // called when the server tells the client to start searching
    private void onState(KalahState ks) throws IOException
    {
        serverSaidStop = false;

        try {
            agent.search(ks);
        }
        catch(Exception e)
        {
            throw new IOException("Exception during agent search: " + e.getMessage());
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
                String msg = receiveFromServer();
                if (msg.equals("stop")) {
                    // server told us to stop

                    // agent already stopped (yielded), do nothing
                    break;
                } else if (msg.equals("ping")) {
                    sendToServer("pong");
                } else if (isErrorMessage(msg)) {
                    throw new IOException("Didn't expect " + msg + " here");
                } else if (msg.equals("goodbye")) {
                    throw new IOException("Server said goodbye");
                }
                else
                {
                    throw new IOException("Got " + msg + ", unknown or malformed message, other than ping/error/goodbye/stop while waiting for stop\"");
                }
            }
        }
    }

    // see documentation of onState(...)
    boolean shouldStop() throws IOException
    {
        if (serverSaidStop)
        {
            return true;
        }
        else if (input.ready())
        {
            String msg = receiveFromServer();
            if (msg.equals("stop"))
            {
                // set stop variable for subsequent calls
                serverSaidStop = true;

                // tell agent to stop
                return true;
            }
            else if (msg.equals("ping"))
            {
                sendToServer("pong");
            }
            else if (isErrorMessage(msg))
            {
                throw new IOException("Exception during agent search: " + msg);
            }
            else if (msg.equals("goodbye"))
            {
                throw new IOException("Server said goodbye");
            }
            else
            {
                throw new IOException("Got " + msg + ", unknown or malformed message, other than ping/error/goodbye/stop while checking for stop\"");
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
    void sendMove(int move)
    {
        sendToServer("move "+move);
    }

    private void sendGoodbyeAndCloseConnection() throws IOException
    {
        sendToServer("goodbye");
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
    private void sendToServer(String msg)
    {
        System.err.println("Client: " + msg);
        output.print(msg + "\r\n");
        output.flush();
    }

    private Command receiveFromServer() throws IOException
    {
        String msg = input.readLine();
        System.err.println("Server: " + msg);

        Matcher mat = compat.matcher(msg);
        int id = Integer.parseInt(mat.group(1));
        int ref = Integer.parseInt(mat.group(2));
        String name = mat.group(3);
        List<String> args = new LinkedList<>();

        int i = 0;
        Matcher arg = argpat.matcher(mat.group(4));
        while (arg.find(i)) {
            args.add(arg.group());
            i = arg.end();
        }

        return new Command(id, ref, name, args);
    }

    // sends error message to server
    private void sendError(String msg)
    {
        sendToServer("error \"" + msg + "\"");
    }

    // checks whether a message is a valid error message
    // error "my error message without newlines or quotes"
    //
    // error "<message>"
    // message doesn't contain any quotation marks
    // message doesn't contain any newlines
    private boolean isErrorMessage(String msg)
    {
        return msg.startsWith("error\"") &&
                msg.endsWith("\"") &&
                !msg.substring(7, msg.length()-1).contains("\"") &&
                !msg.substring(7, msg.length()-1).contains("\n");
    }

    // Checks whether a message is a valid version message
    // kgp <int >= 0> <int >= 0> <int >= 0>
    private boolean isVersionMessage(String msg)
    {
        if (!msg.startsWith("kgp "))
        {
            return false;
        }

        String[] sp = msg.split(" ");

        if (sp.length != 4)
        {
            return false;
        }

        try
        {
            for(int i=1;i<=3;i++) {
                if (Integer.parseInt(sp[i]) < 0)
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

    // checks whether a message is a valid state message
    // state <boardSize, storeSouth, houseSouth1, houseSouth2, ..., houseNorth1, houseNorth2, ...>
    private boolean isStateMessage(String msg) {

        // correct start and ending
        if (!msg.startsWith("state <") ||
                !msg.endsWith(">")
        ) {
            return false;
        }

        // inner message consists only of digits and comma
        String integerString = msg.substring(7, msg.length() - 1);
        for (char c : integerString.toCharArray()) {
            switch (c) {
                case '0':
                case '1':
                case '2':
                case '3':
                case '4':
                case '5':
                case '6':
                case '7':
                case '8':
                case '9':
                case ',':
                    break;

                default:
                    return false;
            }
        }

        // all integers non-negative?
        String[] sp = integerString.split(",");

        // too small even for boardSize 1?
        // boardSize, storeSouth, storeNorth, houseSouth1, houseSouthNorth
        if (sp.length < 5)
        {
            return false;
        }

        for (String s : sp) {
            if (Integer.parseInt(s) < 0) {
                return false;
            }
        }

        // finally: does number of integers fit boardSize?
        int boardSize = Integer.parseInt(sp[0]);
        return boardSize * 2 + 3 == sp.length;
    }

    // TODO ERROR value might be string which contains spaces therefore parsing is wrong
    // checks whether the given message is a valid set message
    private boolean isSetMessage(String msg)
    {
        return msg.startsWith("set ") && msg.split(" ").length == 3;
    }
}
