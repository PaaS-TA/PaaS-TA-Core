package szxcvbn;

import java.util.List;

/**
 * Simple helper to provide Java-friendly function calls
 */
public final class ZxcvbnHelper {

    public static Zxcvbn zxcvbn(String password) {
        return Zxcvbn$.MODULE$.apply(password);
    }

    public static Zxcvbn zxcvbn(String password, List<Matcher<Match>> customMatchers) {
        return Zxcvbn$.MODULE$.apply(password, scala.collection.JavaConversions.asScalaBuffer(customMatchers));
    }

    public static Matcher<Match> createMatcher(String name, List<String> wordList) {
        return Zxcvbn$.MODULE$.createMatcher(name, scala.collection.JavaConversions.asScalaBuffer(wordList));
    }

    public static List<Matcher<Match>> defaultMatchers() {
        return scala.collection.JavaConversions.asJavaList(Zxcvbn$.MODULE$.defaultMatchers());
    }
}
