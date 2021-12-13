package kgp.info.kwarc.kalah.jkpg;

import java.io.*;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.WebSocket;
import java.util.concurrent.CompletionStage;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.LinkedBlockingQueue;


public class ConnectionWebSocket implements Connection {

    private static final String POISON_PILL = "Poison pill, compared by memory address";

    private String host; // including protocol

    private WebSocket webSocket;
    private LinkedBlockingQueue<String> input = new LinkedBlockingQueue<>();

    public ConnectionWebSocket(String host) throws IOException {

        this.host = host;

        WebSocket.Listener listener = new WebSocket.Listener() {

            @Override
            public void onError(WebSocket webSocket, Throwable error) {
                error.printStackTrace();
                System.err.println(error.getMessage());
            }

            @Override
            public CompletionStage<?> onText(WebSocket webSocket, CharSequence data, boolean last) {

                if (!last) {
                    throw new IllegalArgumentException("last was false, message incomplete");
                }

                if (data.charAt(data.length()-1) == '\n') {
                    data = data.subSequence(0, data.length()-1);
                }

                try {
                    System.out.println("FROM WEBSOCKET: " + data);
                    input.add(data.toString());
                } catch (Exception e) {
                    e.printStackTrace();
                    System.exit(1);
                }

                return null;
            }

            @Override
            public CompletionStage<?> onClose(WebSocket webSocket, int statusCode, String reason) {
                System.out.println("Code: "+statusCode);
                System.out.println("Reason: "+reason);
                input.add(POISON_PILL);
                return null;
            }

        };

        try {
            webSocket = HttpClient.newHttpClient().newWebSocketBuilder().buildAsync(URI.create(host), listener).get();
        } catch(ExecutionException ee) {
            throw new IOException("ExecutionException: " + ee.getMessage());
        } catch(InterruptedException ie) {
            throw new IOException("InterruptedException: " + ie.getMessage());
        }

        send("ok");
    }

    @Override
    public void send(String msg) {
        webSocket.sendText(msg + "\n", true);
        //System.out.println("TO WEBSOCKET: " + msg);
    }

    @Override
    public String receive() throws InterruptedException {
        String msg = input.take();
        if (msg == POISON_PILL) {
            return null;
        } else {
            return msg;
        }
    }

    @Override
    public void close() {
        webSocket.sendClose(WebSocket.NORMAL_CLOSURE, "");
    }

}
