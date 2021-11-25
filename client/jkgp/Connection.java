package info.kwarc.kalah.jkpg;


import java.io.IOException;

public interface Connection {

    void send(String msg) throws IOException;
    String receive() throws IOException, InterruptedException;
    void close() throws IOException;

}
