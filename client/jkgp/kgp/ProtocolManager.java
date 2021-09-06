package kgp;

import java.io.*;
import java.net.*;

// THIS PROTOCOL ASSUMES A CORRECT SERVER

public class ProtocolManager {

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
        clientSocket = new Socket(host, port);
        input = new BufferedReader(new InputStreamReader(clientSocket.getInputStream()));
        output = new PrintStream(clientSocket.getOutputStream(), true);

        keepConnection = true;

        try {
            while (keepConnection) {
                String msg = receiveFromServer();

                if (msg.startsWith("ping")) {
                    sendToServer("pong");
                } else if (msg.startsWith("error \"")) {
                    keepConnection = false;
                } else if (msg.equals("goodbye")) {
                    keepConnection = false;
                } else if (msg.startsWith("kgp ")) {
                    if (!msg.startsWith("kgp 1 ")) {
                        // wrong protocol version
                        sendToServer("error \"Only kgp 1 * * supported\"");
                    } else {
                        // supported version, reply with mode
                        sendToServer("mode simple");
                    }
                } else if (msg.startsWith("init ")) {
                    int boardSize = Integer.parseInt(msg.substring(5));

                    // initialize
                    onInit(boardSize);

                    // then tell the server that you're done initializing
                    sendToServer("ok");
                } else if (msg.startsWith("state ")) {
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
            }
        }
        catch(IOException e)
        {
            sendToServer("error \""+e.getMessage()+"\"");
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
        agent.init(boardSize);
    }

    // called when the server tells the client to start searching
    private void onState(KalahState ks) throws IOException
    {
        serverSaidStop = false;

        agent.search(ks);

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
                } else if (msg.startsWith("error \"")) {
                    keepConnection = false;
                    return;
                } else if (msg.equals("goodbye")) {
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
            else if (msg.startsWith("ping"))
            {
                sendToServer("pong");
            }
            else if (msg.startsWith("error \""))
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

        return msg;
    }

}