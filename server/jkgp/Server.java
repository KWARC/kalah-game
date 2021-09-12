package server;

import kgp.KalahState;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.io.PrintStream;
import java.net.ServerSocket;
import java.net.Socket;

// Minimal server implementation:
//  -waits for two clients to connect
//  -plays one game, terminates afterwards
//  -prints the states
//  -doesn't use ping pongs, doesn't react to client errors
//  -no timeout except not accepting new moves after TIME_MOVE has passed
//  -assumes clients cooperate with protocol 1.0.0 perfectly
//  -assumes clients want to play simple mode
//  -assumes clients don't crash or send errors
public class Server {

    private static final int BOARD_SIZE = 4;
    private static final int SEEDS = 4; // Initial seeds in each house
    private static final int TIME_MOVE = 500; // Amount of time for the clients to make one move

    // send given message (without linebreak) to client using the given PrintStream
    // adds CRLF to the message, flushes the PrintStream afterwards
    private static void sendToClient(PrintStream ps, String msg)
    {
        ps.print(msg + "\r\n");
        ps.flush();
    }

    // converts the given state to the Kalah Game Protocol's state command
    private static String stateToProtocolString(KalahState ks)
    {
        StringBuilder s = new StringBuilder("state <");
        s.append(ks.getBoardSize()).append(',');
        s.append(ks.getStoreSouth()).append(',');
        s.append(ks.getStoreNorth());

        for(int i=0;i<ks.getBoardSize();i++)
        {
            s.append(",").append(ks.getHouse(KalahState.Side.SOUTH, i));
        }

        for(int i=0;i<ks.getBoardSize();i++)
        {
            s.append(",").append(ks.getHouse(KalahState.Side.NORTH, i));
        }

        return s + ">";
    }

    public static void main(String[] args) throws IOException {

        // listen for incoming connections and add them to a list
        ServerSocket serverSocket = new ServerSocket(2671);

        // wait for the two clients
        System.out.println("Waiting for north to connect");
        Socket socketNorth = serverSocket.accept();

        System.out.println("Waiting for south to connect");
        Socket socketSouth = serverSocket.accept();

        // we don't want to accept new connections
        serverSocket.close();

        // get input and output channels to both clients
        BufferedReader inputNorth = new BufferedReader(new InputStreamReader(socketNorth.getInputStream()));
        PrintStream outputNorth = new PrintStream(socketNorth.getOutputStream(), true);

        BufferedReader inputSouth = new BufferedReader(new InputStreamReader(socketSouth.getInputStream()));
        PrintStream outputSouth = new PrintStream(socketSouth.getOutputStream(), true);

        // tell the clients about your protocol version
        sendToClient(outputNorth, "kgp 1 0 0");
        sendToClient(outputSouth, "kgp 1 0 0");

        // consume "mode simple" messages from clients
        inputNorth.readLine();
        inputSouth.readLine();

        // create new board of size BOARD_SIZE with SEEDS in each house
        KalahState ks = new KalahState(BOARD_SIZE, SEEDS);

        // print the initial state
        System.out.println(ks + "\n");

        // while the game is not over
        while (ks.result() != KalahState.GameResult.WIN &&
                ks.result() != KalahState.GameResult.DRAW &&
                ks.result() != KalahState.GameResult.LOSS) {

            // Channels of the client to move
            PrintStream outputMover;
            BufferedReader inputMover;

            // flip the board, so it's south to move because the protocol assumes that it's south to move
            boolean wasFlipped = ks.flipIfNorthToMove();

            if (wasFlipped)
            {
                outputMover = outputNorth;
                inputMover = inputNorth;
            }
            else
            {
                outputMover = outputSouth;
                inputMover = inputSouth;
            }

            // tell that client to start calculating
            String state_msg = stateToProtocolString(ks);
            sendToClient(outputMover, state_msg);

            // remember when the search started
            long start_time = System.currentTimeMillis();

            // handle incoming moves while there's still time
            int chosenMove = -1;
            boolean yielded = false;
            while (System.currentTimeMillis() - start_time < TIME_MOVE)
            {
                // if there is a new message, process it
                if (inputMover.ready()) {
                    String msg = inputMover.readLine();
                    if (msg.startsWith("move ")) {
                        // if it's a move, update the chosen move
                        chosenMove = Integer.parseInt(msg.substring(5));
                    } else if (msg.equals("yield")) {
                        // if the agent yielded, remember that act as if time was up
                        yielded = true;
                        break;
                    }
                }
                else
                {
                    // otherwise, sleep for 10ms, yep it's dirty,
                    // but I couldn't find a suitable blocking method with timeout

                    try
                    {
                        Thread.sleep(10);
                    } catch(InterruptedException e)
                    {
                        e.printStackTrace();
                    }
                }
            }

            // tell client to stop in either case (as specified in the Kalah Game Protocol 1.0.0)
            // since the server might send a stop before receiving a yield etc.
            sendToClient(outputMover, "stop");

            if (!yielded) // if client hasn't yielded, we need to wait for a yield or ok
            {
                // wait for "ok" or "yield"
                while (true)
                {
                    String msg = inputMover.readLine();
                    if (msg.equals("ok") || msg.equals("yield"))
                        break;
                    else
                    {
                        // consume move message by doing nothing, moves are not accepted after the time is up
                    }
                }
            }

            // Is the move illegal?
            if (chosenMove <= 0 || !ks.getMoves().contains(chosenMove - 1))
            {
                // notify both client of the error
                sendToClient(outputNorth, "error \"Someone sent an illegal move or no move\"");
                sendToClient(outputSouth, "error \"Someone sent an illegal move or no move\"");

                // break the loop (and thus say goodbye and end the connections)
                break;
            }

            // execute that move, subtract -1 because the KalahState class indexes moves from 0 to board_size-1
            ks.doMove(chosenMove - 1);

            ks.flipIfWasFlipped(wasFlipped);

            System.out.println("Move " + chosenMove + "\n");

            System.out.println(ks + "\n");
        }

        // say goodbye to both clients (tell them to end the connection)
        sendToClient(outputNorth, "goodbye");
        sendToClient(outputSouth, "goodbye");

        // then close the connections
        inputNorth.close();
        inputSouth.close();

        outputNorth.close();
        outputSouth.close();

        socketNorth.close();
        socketSouth.close();
    }

}