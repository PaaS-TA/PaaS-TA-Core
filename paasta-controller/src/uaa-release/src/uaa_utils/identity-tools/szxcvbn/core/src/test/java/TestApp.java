import static szxcvbn.ZxcvbnHelper.*;

public class TestApp {

    public static void main(String[] args) {
        long end = System.currentTimeMillis() + 5*60*1000;

        while (System.currentTimeMillis() < end) {
            zxcvbn("a");
            zxcvbn("aaaaaa");
            zxcvbn("password");
            zxcvbn("coRrecth0rseba++ery9.23.2007staple$");
        }
    }
}
