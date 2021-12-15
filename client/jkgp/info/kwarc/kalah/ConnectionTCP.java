package info.kwarc.kalah;

import java.io.BufferedReader;
import java.io.IOException;
import java.io.InputStreamReader;
import java.io.OutputStream;
import java.net.Socket;

public class ConnectionTCP implements Connection {

    private String host;
    private int port;

    private Socket clientSocket;
    private BufferedReader input;
    private OutputStream output;

    public ConnectionTCP(String host, int port) throws IOException {
        this.host = host;
        this.port = port;

        clientSocket = new Socket(host, port);

        input = new BufferedReader(new InputStreamReader(clientSocket.getInputStream()));
        output = clientSocket.getOutputStream();
    }

    @Override
    public void send(String msg) throws IOException {
        output.write(msg.getBytes());
        output.write('\r');
        output.write('\n');
        output.flush();
    }

    @Override
    public String receive() throws IOException {
        return input.readLine();
    }

    @Override
    public void close() throws IOException {
        input.close();
        output.close();
        clientSocket.close();
    }
}
