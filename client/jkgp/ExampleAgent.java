package kgp;

import java.io.IOException;
import java.math.BigInteger;
import java.util.ArrayList;
import java.util.Random;

// simple example of an agent
// chooses among the legal moves uniform at random, sends new "best" moves in increasing intervals
// the latter is just to make it a better example, we don't want to provide any algorithms here
public class ExampleAgent extends Agent {

    // toy example rsa key, public so it can be used for debugging elsewhere
    // valid 4096 bit RSA key, generated with python
    // pip install rsa
    // https://stuvel.eu/python-rsa-doc/usage.html#generating-keys
    // Access e, d via privkey.e and privkey.d, calculate N as privkey.p * privkey.q
    public static final BigInteger N = new BigInteger("858136784250475827739667800182405090386511966134503458688009322373334850610073269688975997353180793606031097468936781197238335725131229850833235296298419987548943606192384070588013786696173721335000963082090099891260840112130841126650261373263939427189508205895640732288611397467302583687177280899143702058150829722119489198647470803943918490494167013275456062518408579733305744294364079258165495434444566524198809722820086020281756649993298858400530671845986492456070587763100418983896274178217954736081735864708165389698527164760979666184233139092364857996960503664595034965980583209317210262050820300481445818510195526519720170771279657551532401036642253990040618416358500914122466078177295747666659444554745182474390266417189090140423110823235069661902488592707212170453184961574960526461678398234347399585575790259377207428631253681122436231722629814113580092303059730736527411158015728748705627757287424980208446389716068378085789054494543541310876647646777722014171983775861532948556729949639076751292720649873337122118255547576024781422069190765249001358905734656376097560502585586309025135390608560279267881740528988985795811911166450860127285920999294106532502393216309492417834797730432220257028957894886522818112117813801");
    public static final BigInteger e = new BigInteger("65537");
    public static final BigInteger d = new BigInteger("541054214596547166913816823646751617252255381733125065481288939221944388087017067867283781476582443087031920571798171046143175160568038644830860669215054279346169320406419307883597321819317245819667894403391138099192657190188114899893425854168473397788999627322180916106897043727152754330192905138067304160166782656326951388945361262947139111428803197515222240250898895633915599737360851412586118327764223772013046318211831094558226139957170790972554860481071880842536166699102375953027723962313564973215640137107376017634814547127400411773482117589081103762644078488864785385795214198914005799157439637962889861016644857826972851562716281749811201119211316112660695606747292510657951254495973920141062952076008056230533627544125391136664437057212921121027654327245357873988508199327329863968073882639080688831557727970967130074011551897855432367176486605699606598483855282662424517223570945683767719647423320210114406297640450218189383938228255200454455676746855544807542593458540656332687279128036630116729998200414817540152397528349434568557998806001025844746161211194429427180872266965338948337734426823901791743220418850419389743414841155778815020080793049014351624243476468480429182184163249357404913367903916253906247882113269");

    private final Random rng;

    public ExampleAgent(String host, int port) {

        super(
                host,
                port,
                "ExampleAgentName",
                "Philip Kaludercic, Tobias Völk",
                "Sophisticated Kalah agent developed by Philip Kaludercic und Tobias Völk in 2021.\n\n" +
                        "Chooses among the legal moves uniform at random.\n" +
                        "Very friendly to the environment.",
                N,
                e,
                d
        );
        // Initialize your agent, load databases, neural networks, ...
        rng = new Random();
    }

    @Override
    public void search(KalahState ks) throws IOException {

        // Immediately send some legal move in case time runs out early
        submitMove(ks.lowestLegalMove());

        // The actual "search". ShouldStop is checked in a loop but if you're doing a recursive search you might want
        // to check it every N nodes or every N milliseconds, just so it's called a few times per second,
        // as a good server punishes slow reactions to the stop command by subtracting the delay from the amount of
        // time for the next move

        long timeToWait = 50;
        while (!shouldStop())
        {
            // pick a random move
            ArrayList<Integer> moves = ks.getMoves();
            int randomIndex = rng.nextInt(moves.size());
            int chosenMove = moves.get(randomIndex);

            // send that move to the server
            this.submitMove(chosenMove);

            // Commenting on the current position and/or move choice
            sendComment("I chose move " + (chosenMove + 1) + " because the RNG told me to so.\n" +
                    "evaluation: " + "How am I supposed to know??\n" +
                    "\"You shouldn't have played that move, you're DOOOMED!");

            sleep(timeToWait);

            timeToWait *= 2.0; // increase search time
        }

        // This implementation doesn't return from search() until the server says so,
        // but that would be perfectly fine, for example if your agent found a proven win
    }

    // you can also implement your own methods of course
    private static void sleep(long millis)
    {
        long start = System.currentTimeMillis();
        while(System.currentTimeMillis() < start + millis) {
            try {
                long remainingTime = (start + millis) - System.currentTimeMillis();
                Thread.sleep(remainingTime);
            } catch (InterruptedException e) {
                e.printStackTrace();
            }
        }
    }

    // Example of a main function
    public static void main(String[] args) throws IOException {

        // Prepare agent for playing on a server on, for example, the same machine
        // Agent initialization happens before we connect to the server
        // Not that tournament programs might start your client in a process and punish it
        // if it doesn't connect to the server within a specified amount of time
        // 2671 is the Kalah Game Protocol default port
        Agent agent = new ExampleAgent("localhost", 2671);

        // If necessary, do some other stuff here before connecting.
        // The game might start immediately after connecting!

        // Connects to the server, plays the tournament / game, ends the connection. Handles everything.
        agent.run();
    }

}