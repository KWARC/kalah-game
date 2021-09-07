package kgp;

import java.io.*;
import java.net.*;

// This protocol implementation is kinda robust
// Its only weakness is the acceptance of any remaining message after a correct state message
// Shuts down cleanly in case of Exceptions (client crashes), passes on the Exception
// Notifies the server of the server's protocol errors (wrong message / at the wrong time),
// agent crashes and other exceptions

public class ProtocolManager {

    private enum ProtocolState
    {
        WAITING_FOR_VERSION,
        WAITING_FOR_INIT,
        INITIALIZED,
    }

    private final String host;
    private final int port;
    private final Agent agent;

    private Socket clientSocket;
    private BufferedReader input;
    private PrintStream output;

    private boolean serverSaidStop, keepConnection;

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

        // state of the protocol state
        ProtocolState state = ProtocolState.WAITING_FOR_VERSION;

        keepConnection = true;

        try {
            while (keepConnection) {
                String msg = receiveFromServer();

                if (msg.equals("ping")) {
                    sendToServer("pong");
                } else if (isErrorMessage(msg)) {
                    keepConnection = false; // break
                } else if (msg.equals("goodbye")) {
                    keepConnection = false; // break
                } else if (isVersionMessage(msg)) {
                    if (state == ProtocolState.WAITING_FOR_VERSION) {
                        if (!msg.startsWith("kgp 1 ")) {
                            // wrong protocol version
                            sendError("Only kgp 1.*.* supported");
                            keepConnection = false; // break
                        } else {
                            // supported version, reply with mode
                            sendToServer("mode simple");

                            // and wait for initialization message
                            state = ProtocolState.WAITING_FOR_INIT;
                        }
                    }
                    else
                    {
                        sendError("Didn't expect " + msg + " here");
                        keepConnection = false; // break
                    }
                } else if (isInitMessage(msg)) {
                    if (state == ProtocolState.WAITING_FOR_INIT) {
                        // isInitMessage already checks the integer
                        int boardSize = Integer.parseInt(msg.substring(5));

                        // initialize
                        onInit(boardSize);

                        // then tell the server that you're done initializing
                        sendToServer("ok");

                        state = ProtocolState.INITIALIZED;
                    }
                    else
                    {
                        sendError("Didn't expect " + msg + " message here");
                        keepConnection = false; // break
                    }
                } else if (isStateMessage(msg)) {
                    if (state == ProtocolState.INITIALIZED) {
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
                    else
                    {
                        sendError("Didn't expect " + msg +" here");
                        keepConnection = false; // break
                    }
                }
                else
                {
                    sendError("Unknown (slightly wrong?) message " + msg);
                    keepConnection = false; // break
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

    // called when the server tells the client to initialize
    private void onInit(int boardSize)
    {
        try {
            agent.init(boardSize);
        }
        catch(Exception e)
        {
            sendError("Exception during agent initialization: " + e.getMessage());
            keepConnection = false;
        }
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
            sendError("Exception during agent search: " + e.getMessage());
            keepConnection = false;
            return;
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
                    keepConnection = false;
                    return;
                } else if (msg.equals("goodbye")) {
                    keepConnection = false;
                    return;
                }
                else
                {
                    sendError("Got "+msg+" (other than ping/error/goodbye/stop while waiting for stop)");
                    keepConnection = false;
                    return;
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
                // error, tell agent to stop
                keepConnection = false;
                return true;
            }
            else if (msg.equals("goodbye"))
            {
                // server suddenly ended connection, tell agent to stop
                keepConnection = false;
                return true;
            }
            else
            {
                sendError("Got "+msg+" other than ping/error/goodbye/stop while waiting for stop\"");
                keepConnection = false;
                return true;
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

    // do callback stuff like logging when receiving message
    private String receiveFromServer() throws IOException
    {
        String msg = input.readLine();
        System.err.println("Server: " + msg);

	// strip the command ID and reference from msg
	int i = 0, s = 0;	// index, state
	loop: while (s != 5) {
	    switch (s) {
	    case 0:		// beginning of line
		switch (msg.charAt(i)) {
		case ' ': case '\t':
		    // leading whitespace is ignored
		    break;
		case '0': case '1': case '2': case '3': case '4':
		case '5': case '6': case '7': case '8': case '9':
		    // a possible command ID has been found
		    s = 1;
		    break;
		default:	// something else
		    s = -1;
		    break loop;
		}
		break;
	    case 1:		// in command ID
		switch (msg.charAt(i)) {
		case ' ': case '\t':
		    s = 4;
		    break;
		case '@':	// reference ID expected
		    s = 2;
		    break;
		case '0': case '1': case '2': case '3': case '4':
		case '5': case '6': case '7': case '8': case '9':
		    // continue parsing command ID
		    break;
		default:	// something else
		    s = -1;
		    break loop;
		}
		break;
	    case 2:		// expecting reference
		switch (msg.charAt(i)) {
		case '0': case '1': case '2': case '3': case '4':
		case '5': case '6': case '7': case '8': case '9':
		    state = 3;
		    break;
		default:	// something else
		    s = -1;
		    break loop;
		}
	    case 3:		// in command reference
		switch (msg.charAt(i)) {
		case ' ': case '\t':
		    s = 4;
		    break;
		case '0': case '1': case '2': case '3': case '4':
		case '5': case '6': case '7': case '8': case '9':
		    // continue parsing command reference
		    break;
		default:
		    s = -1;
		    break loop;
		}
	    case 4:		// ID and reference have been parsed
		switch (msg.charAt(i)) {
		case ' ': case '\t':
		    // Any trailing whitespace after the ID or the
		    // command reference is jumped over
		    break;
		default:	// finished parsing
		    break loop;
		}
	    }
	    i++;
	}
	if (s == -1) {
	    throw new IOException("Protocol error")
	}
	
        return msg.substring(i);
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

    // Checks whether a message is a valid init message
    // init <int >= 1>
    private boolean isInitMessage(String msg)
    {
        if (!msg.startsWith("init "))
        {
            return false;
        }

        String[] sp = msg.split(" ");
        if (sp.length != 2)
        {
            return false;
        }

        try
        {
            if (Integer.parseInt(sp[1]) < 1)
            {
                return false;
            }
        }
        catch(NumberFormatException e)
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
}
