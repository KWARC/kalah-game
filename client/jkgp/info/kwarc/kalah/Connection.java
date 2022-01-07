package info.kwarc.kalah;


import java.io.IOException;

interface Connection {

    void send(String msg) throws IOException;

    String receive() throws IOException, InterruptedException;

    void close() throws IOException;

}
