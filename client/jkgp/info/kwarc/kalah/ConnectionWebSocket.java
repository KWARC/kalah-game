package info.kwarc.kalah;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.WebSocket;
import java.util.concurrent.CompletionStage;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.LinkedBlockingQueue;


class ConnectionWebSocket implements Connection {

    private static final String POISON_PILL = "Poison pill, compared by memory address";

    private String host; // including protocol

    private WebSocket webSocket;
    private LinkedBlockingQueue<String> input = new LinkedBlockingQueue<>();

    private String acc = ""; // for accumulating WebSocket messages if they arrive in parts

    ConnectionWebSocket(String host) throws IOException {

        this.host = host;

        WebSocket.Listener listener = new WebSocket.Listener() {

            @Override
            public CompletionStage<?> onText(WebSocket webSocket, CharSequence data, boolean last) {

                acc = acc + data;

                if (last) {
                    if (acc.charAt(data.length() - 1) == '\n') {
                        acc = (String) acc.subSequence(0, acc.length() - 1);
                    }

                    try {
                        input.add(acc);
                    } catch (Exception e) {
                        e.printStackTrace();
                        System.exit(1);
                    }
                    acc = "";
                }

                webSocket.request(1);
                return null;
            }

            @Override
            public CompletionStage<?> onClose(WebSocket webSocket, int statusCode, String reason) {
                input.add(POISON_PILL);
                return null;
            }

        };

        try {
            webSocket = HttpClient.newHttpClient().newWebSocketBuilder().buildAsync(URI.create(host), listener).get();
        } catch (ExecutionException ee) {
            throw new IOException("ExecutionException: " + ee.getMessage());
        } catch (InterruptedException ie) {
            throw new IOException("InterruptedException: " + ie.getMessage());
        }
    }

    @Override
    public void send(String msg) {
        webSocket.sendText(msg + "\n", true);
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
