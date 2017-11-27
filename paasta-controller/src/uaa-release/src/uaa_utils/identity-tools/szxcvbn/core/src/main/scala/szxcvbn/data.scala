package szxcvbn

import scala.math._
import szxcvbn.Predef._
import scala.Some

final case class AdjacencyGraph(name: String, graph: Map[Char, Seq[String]]) {
  private val startPositions = graph.size
  private val avgDegree = graph.foldLeft(0.0)((s, kv) => { s + kv._2.count(_ != null)}) / startPositions

//  def get(key: Char) = graph.get(key)

  /**
   * Determines whether c2 is an adjacent character of c1 in the graph and returns
   * a pair containing the direction and whether the adjacent character is shifted or not.
   *
   * Returns `None` if c1 is not in the graph or c2 is not one of its adjacents.
   */
  def adjacentMatch(c1: Char, c2: Char): Option[(Int, Boolean)] = {
    graph.get(c1) match {
      case None => None
      case Some(s) => findAdjacent(c2, s)
    }
  }

  private def findAdjacent(c: Char, adjacents: Seq[String], direction: Int = 0): Option[(Int,Boolean)] =
    if (direction == adjacents.length)
      None
    else
      adjacents(direction) match {
        case null =>
          findAdjacent(c, adjacents, direction + 1)
        case a =>
          a.indexOf(c) match {
            case -1 => findAdjacent(c, adjacents, direction + 1)
            case i => Some(direction, i == 1)
          }
      }

  // TODO: Pre-calculate these for, e.g. l in 1..10?
  def entropy(length: Int, turns: Int, nShifted: Int) =
    log2((1 until length).foldLeft(0.0)(_ + possibilities(_, turns))) + shiftEntropy(length, nShifted)

  private def possibilities(l: Int, turns: Int) =
    (1 to min(turns, l)).foldLeft(0.0)((t,j) => t + nCk(l, j-1) * startPositions * pow(avgDegree, j))

  private def shiftEntropy(length: Int, nShifted: Int) = if (nShifted == 0) 0.0 else
    log2((0 to min(nShifted, length - nShifted)).foldLeft(0.0)(_ + nCk(length, _)))


}

object Data {
  val adjacencyGraphs = List(
    AdjacencyGraph("qwerty", Map('!' -> Seq("`~", null, null, "2@", "qQ", null), '"' -> Seq(";:", "[{", "]}", null, null, "/?"), '#' -> Seq("2@", null, null, "4$", "eE", "wW"), '$' -> Seq("3#", null, null, "5%", "rR", "eE"), '%' -> Seq("4$", null, null, "6^", "tT", "rR"), '&' -> Seq("6^", null, null, "8*", "uU", "yY"), '\'' -> Seq(";:", "[{", "]}", null, null, "/?"), '(' -> Seq("8*", null, null, "0)", "oO", "iI"), ')' -> Seq("9(", null, null, "-_", "pP", "oO"), '*' -> Seq("7&", null, null, "9(", "iI", "uU"), '+' -> Seq("-_", null, null, null, "]}", "[{"), ',' -> Seq("mM", "kK", "lL", ".>", null, null), '-' -> Seq("0)", null, null, "=+", "[{", "pP"), '.' -> Seq(",<", "lL", ";:", "/?", null, null), '/' -> Seq(".>", ";:", "'\"", null, null, null), '0' -> Seq("9(", null, null, "-_", "pP", "oO"), '1' -> Seq("`~", null, null, "2@", "qQ", null), '2' -> Seq("1!", null, null, "3#", "wW", "qQ"), '3' -> Seq("2@", null, null, "4$", "eE", "wW"), '4' -> Seq("3#", null, null, "5%", "rR", "eE"), '5' -> Seq("4$", null, null, "6^", "tT", "rR"), '6' -> Seq("5%", null, null, "7&", "yY", "tT"), '7' -> Seq("6^", null, null, "8*", "uU", "yY"), '8' -> Seq("7&", null, null, "9(", "iI", "uU"), '9' -> Seq("8*", null, null, "0)", "oO", "iI"), ':' -> Seq("lL", "pP", "[{", "'\"", "/?", ".>"), ';' -> Seq("lL", "pP", "[{", "'\"", "/?", ".>"), '<' -> Seq("mM", "kK", "lL", ".>", null, null), '=' -> Seq("-_", null, null, null, "]}", "[{"), '>' -> Seq(",<", "lL", ";:", "/?", null, null), '?' -> Seq(".>", ";:", "'\"", null, null, null), '@' -> Seq("1!", null, null, "3#", "wW", "qQ"), 'A' -> Seq(null, "qQ", "wW", "sS", "zZ", null), 'B' -> Seq("vV", "gG", "hH", "nN", null, null), 'C' -> Seq("xX", "dD", "fF", "vV", null, null), 'D' -> Seq("sS", "eE", "rR", "fF", "cC", "xX"), 'E' -> Seq("wW", "3#", "4$", "rR", "dD", "sS"), 'F' -> Seq("dD", "rR", "tT", "gG", "vV", "cC"), 'G' -> Seq("fF", "tT", "yY", "hH", "bB", "vV"), 'H' -> Seq("gG", "yY", "uU", "jJ", "nN", "bB"), 'I' -> Seq("uU", "8*", "9(", "oO", "kK", "jJ"), 'J' -> Seq("hH", "uU", "iI", "kK", "mM", "nN"), 'K' -> Seq("jJ", "iI", "oO", "lL", ",<", "mM"), 'L' -> Seq("kK", "oO", "pP", ";:", ".>", ",<"), 'M' -> Seq("nN", "jJ", "kK", ",<", null, null), 'N' -> Seq("bB", "hH", "jJ", "mM", null, null), 'O' -> Seq("iI", "9(", "0)", "pP", "lL", "kK"), 'P' -> Seq("oO", "0)", "-_", "[{", ";:", "lL"), 'Q' -> Seq(null, "1!", "2@", "wW", "aA", null), 'R' -> Seq("eE", "4$", "5%", "tT", "fF", "dD"), 'S' -> Seq("aA", "wW", "eE", "dD", "xX", "zZ"), 'T' -> Seq("rR", "5%", "6^", "yY", "gG", "fF"), 'U' -> Seq("yY", "7&", "8*", "iI", "jJ", "hH"), 'V' -> Seq("cC", "fF", "gG", "bB", null, null), 'W' -> Seq("qQ", "2@", "3#", "eE", "sS", "aA"), 'X' -> Seq("zZ", "sS", "dD", "cC", null, null), 'Y' -> Seq("tT", "6^", "7&", "uU", "hH", "gG"), 'Z' -> Seq(null, "aA", "sS", "xX", null, null), '[' -> Seq("pP", "-_", "=+", "]}", "'\"", ";:"), '\\' -> Seq("]}", null, null, null, null, null), ']' -> Seq("[{", "=+", null, "\\|", null, "'\""), '^' -> Seq("5%", null, null, "7&", "yY", "tT"), '_' -> Seq("0)", null, null, "=+", "[{", "pP"), '`' -> Seq(null, null, null, "1!", null, null), 'a' -> Seq(null, "qQ", "wW", "sS", "zZ", null), 'b' -> Seq("vV", "gG", "hH", "nN", null, null), 'c' -> Seq("xX", "dD", "fF", "vV", null, null), 'd' -> Seq("sS", "eE", "rR", "fF", "cC", "xX"), 'e' -> Seq("wW", "3#", "4$", "rR", "dD", "sS"), 'f' -> Seq("dD", "rR", "tT", "gG", "vV", "cC"), 'g' -> Seq("fF", "tT", "yY", "hH", "bB", "vV"), 'h' -> Seq("gG", "yY", "uU", "jJ", "nN", "bB"), 'i' -> Seq("uU", "8*", "9(", "oO", "kK", "jJ"), 'j' -> Seq("hH", "uU", "iI", "kK", "mM", "nN"), 'k' -> Seq("jJ", "iI", "oO", "lL", ",<", "mM"), 'l' -> Seq("kK", "oO", "pP", ";:", ".>", ",<"), 'm' -> Seq("nN", "jJ", "kK", ",<", null, null), 'n' -> Seq("bB", "hH", "jJ", "mM", null, null), 'o' -> Seq("iI", "9(", "0)", "pP", "lL", "kK"), 'p' -> Seq("oO", "0)", "-_", "[{", ";:", "lL"), 'q' -> Seq(null, "1!", "2@", "wW", "aA", null), 'r' -> Seq("eE", "4$", "5%", "tT", "fF", "dD"), 's' -> Seq("aA", "wW", "eE", "dD", "xX", "zZ"), 't' -> Seq("rR", "5%", "6^", "yY", "gG", "fF"), 'u' -> Seq("yY", "7&", "8*", "iI", "jJ", "hH"), 'v' -> Seq("cC", "fF", "gG", "bB", null, null), 'w' -> Seq("qQ", "2@", "3#", "eE", "sS", "aA"), 'x' -> Seq("zZ", "sS", "dD", "cC", null, null), 'y' -> Seq("tT", "6^", "7&", "uU", "hH", "gG"), 'z' -> Seq(null, "aA", "sS", "xX", null, null), '{' -> Seq("pP", "-_", "=+", "]}", "'\"", ";:"), '|' -> Seq("]}", null, null, null, null, null), '}' -> Seq("[{", "=+", null, "\\|", null, "'\""), '~' -> Seq(null, null, null, "1!", null, null))),
    AdjacencyGraph("dvorak", Map('!' -> Seq("`~", null, null, "2@", "'\"", null), '"' -> Seq(null, "1!", "2@", ",<", "aA", null), '#' -> Seq("2@", null, null, "4$", ".>", ",<"), '$' -> Seq("3#", null, null, "5%", "pP", ".>"), '%' -> Seq("4$", null, null, "6^", "yY", "pP"), '&' -> Seq("6^", null, null, "8*", "gG", "fF"), '\'' -> Seq(null, "1!", "2@", ",<", "aA", null), '(' -> Seq("8*", null, null, "0)", "rR", "cC"), ')' -> Seq("9(", null, null, "[{", "lL", "rR"), '*' -> Seq("7&", null, null, "9(", "cC", "gG"), '+' -> Seq("/?", "]}", null, "\\|", null, "-_"), ',' -> Seq("'\"", "2@", "3#", ".>", "oO", "aA"), '-' -> Seq("sS", "/?", "=+", null, null, "zZ"), '.' -> Seq(",<", "3#", "4$", "pP", "eE", "oO"), '/' -> Seq("lL", "[{", "]}", "=+", "-_", "sS"), '0' -> Seq("9(", null, null, "[{", "lL", "rR"), '1' -> Seq("`~", null, null, "2@", "'\"", null), '2' -> Seq("1!", null, null, "3#", ",<", "'\""), '3' -> Seq("2@", null, null, "4$", ".>", ",<"), '4' -> Seq("3#", null, null, "5%", "pP", ".>"), '5' -> Seq("4$", null, null, "6^", "yY", "pP"), '6' -> Seq("5%", null, null, "7&", "fF", "yY"), '7' -> Seq("6^", null, null, "8*", "gG", "fF"), '8' -> Seq("7&", null, null, "9(", "cC", "gG"), '9' -> Seq("8*", null, null, "0)", "rR", "cC"), ':' -> Seq(null, "aA", "oO", "qQ", null, null), ';' -> Seq(null, "aA", "oO", "qQ", null, null), '<' -> Seq("'\"", "2@", "3#", ".>", "oO", "aA"), '=' -> Seq("/?", "]}", null, "\\|", null, "-_"), '>' -> Seq(",<", "3#", "4$", "pP", "eE", "oO"), '?' -> Seq("lL", "[{", "]}", "=+", "-_", "sS"), '@' -> Seq("1!", null, null, "3#", ",<", "'\""), 'A' -> Seq(null, "'\"", ",<", "oO", ";:", null), 'B' -> Seq("xX", "dD", "hH", "mM", null, null), 'C' -> Seq("gG", "8*", "9(", "rR", "tT", "hH"), 'D' -> Seq("iI", "fF", "gG", "hH", "bB", "xX"), 'E' -> Seq("oO", ".>", "pP", "uU", "jJ", "qQ"), 'F' -> Seq("yY", "6^", "7&", "gG", "dD", "iI"), 'G' -> Seq("fF", "7&", "8*", "cC", "hH", "dD"), 'H' -> Seq("dD", "gG", "cC", "tT", "mM", "bB"), 'I' -> Seq("uU", "yY", "fF", "dD", "xX", "kK"), 'J' -> Seq("qQ", "eE", "uU", "kK", null, null), 'K' -> Seq("jJ", "uU", "iI", "xX", null, null), 'L' -> Seq("rR", "0)", "[{", "/?", "sS", "nN"), 'M' -> Seq("bB", "hH", "tT", "wW", null, null), 'N' -> Seq("tT", "rR", "lL", "sS", "vV", "wW"), 'O' -> Seq("aA", ",<", ".>", "eE", "qQ", ";:"), 'P' -> Seq(".>", "4$", "5%", "yY", "uU", "eE"), 'Q' -> Seq(";:", "oO", "eE", "jJ", null, null), 'R' -> Seq("cC", "9(", "0)", "lL", "nN", "tT"), 'S' -> Seq("nN", "lL", "/?", "-_", "zZ", "vV"), 'T' -> Seq("hH", "cC", "rR", "nN", "wW", "mM"), 'U' -> Seq("eE", "pP", "yY", "iI", "kK", "jJ"), 'V' -> Seq("wW", "nN", "sS", "zZ", null, null), 'W' -> Seq("mM", "tT", "nN", "vV", null, null), 'X' -> Seq("kK", "iI", "dD", "bB", null, null), 'Y' -> Seq("pP", "5%", "6^", "fF", "iI", "uU"), 'Z' -> Seq("vV", "sS", "-_", null, null, null), '[' -> Seq("0)", null, null, "]}", "/?", "lL"), '\\' -> Seq("=+", null, null, null, null, null), ']' -> Seq("[{", null, null, null, "=+", "/?"), '^' -> Seq("5%", null, null, "7&", "fF", "yY"), '_' -> Seq("sS", "/?", "=+", null, null, "zZ"), '`' -> Seq(null, null, null, "1!", null, null), 'a' -> Seq(null, "'\"", ",<", "oO", ";:", null), 'b' -> Seq("xX", "dD", "hH", "mM", null, null), 'c' -> Seq("gG", "8*", "9(", "rR", "tT", "hH"), 'd' -> Seq("iI", "fF", "gG", "hH", "bB", "xX"), 'e' -> Seq("oO", ".>", "pP", "uU", "jJ", "qQ"), 'f' -> Seq("yY", "6^", "7&", "gG", "dD", "iI"), 'g' -> Seq("fF", "7&", "8*", "cC", "hH", "dD"), 'h' -> Seq("dD", "gG", "cC", "tT", "mM", "bB"), 'i' -> Seq("uU", "yY", "fF", "dD", "xX", "kK"), 'j' -> Seq("qQ", "eE", "uU", "kK", null, null), 'k' -> Seq("jJ", "uU", "iI", "xX", null, null), 'l' -> Seq("rR", "0)", "[{", "/?", "sS", "nN"), 'm' -> Seq("bB", "hH", "tT", "wW", null, null), 'n' -> Seq("tT", "rR", "lL", "sS", "vV", "wW"), 'o' -> Seq("aA", ",<", ".>", "eE", "qQ", ";:"), 'p' -> Seq(".>", "4$", "5%", "yY", "uU", "eE"), 'q' -> Seq(";:", "oO", "eE", "jJ", null, null), 'r' -> Seq("cC", "9(", "0)", "lL", "nN", "tT"), 's' -> Seq("nN", "lL", "/?", "-_", "zZ", "vV"), 't' -> Seq("hH", "cC", "rR", "nN", "wW", "mM"), 'u' -> Seq("eE", "pP", "yY", "iI", "kK", "jJ"), 'v' -> Seq("wW", "nN", "sS", "zZ", null, null), 'w' -> Seq("mM", "tT", "nN", "vV", null, null), 'x' -> Seq("kK", "iI", "dD", "bB", null, null), 'y' -> Seq("pP", "5%", "6^", "fF", "iI", "uU"), 'z' -> Seq("vV", "sS", "-_", null, null, null), '{' -> Seq("0)", null, null, "]}", "/?", "lL"), '|' -> Seq("=+", null, null, null, null, null), '}' -> Seq("[{", null, null, null, "=+", "/?"), '~' -> Seq(null, null, null, "1!", null, null))),
    AdjacencyGraph("keypad", Map('*' -> Seq("/", null, null, null, "-", "+", "9", "8"), '+' -> Seq("9", "*", "-", null, null, null, null, "6"), '-' -> Seq("*", null, null, null, null, null, "+", "9"), '.' -> Seq("0", "2", "3", null, null, null, null, null), '/' -> Seq(null, null, null, null, "*", "9", "8", "7"), '0' -> Seq(null, "1", "2", "3", ".", null, null, null), '1' -> Seq(null, null, "4", "5", "2", "0", null, null), '2' -> Seq("1", "4", "5", "6", "3", ".", "0", null), '3' -> Seq("2", "5", "6", null, null, null, ".", "0"), '4' -> Seq(null, null, "7", "8", "5", "2", "1", null), '5' -> Seq("4", "7", "8", "9", "6", "3", "2", "1"), '6' -> Seq("5", "8", "9", "+", null, null, "3", "2"), '7' -> Seq(null, null, null, "/", "8", "5", "4", null), '8' -> Seq("7", null, "/", "*", "9", "6", "5", "4"), '9' -> Seq("8", "/", "*", "-", "+", null, "6", "5"))),
    AdjacencyGraph("mac_keypad", Map('*' -> Seq("/", null, null, null, null, null, "-", "9"), '+' -> Seq("6", "9", "-", null, null, null, null, "3"), '-' -> Seq("9", "/", "*", null, null, null, "+", "6"), '.' -> Seq("0", "2", "3", null, null, null, null, null), '/' -> Seq("=", null, null, null, "*", "-", "9", "8"), '0' -> Seq(null, "1", "2", "3", ".", null, null, null), '1' -> Seq(null, null, "4", "5", "2", "0", null, null), '2' -> Seq("1", "4", "5", "6", "3", ".", "0", null), '3' -> Seq("2", "5", "6", "+", null, null, ".", "0"), '4' -> Seq(null, null, "7", "8", "5", "2", "1", null), '5' -> Seq("4", "7", "8", "9", "6", "3", "2", "1"), '6' -> Seq("5", "8", "9", "-", "+", null, "3", "2"), '7' -> Seq(null, null, null, "=", "8", "5", "4", null), '8' -> Seq("7", null, "=", "/", "9", "6", "5", "4"), '9' -> Seq("8", "=", "/", "*", "-", "+", "6", "5"), '=' -> Seq(null, null, null, null, "/", "9", "8", "7")))
  )

  lazy val dicts = {
    var passwords = readWordList("common_passwords_short.txt")
    var english = readWordList("english.txt")
    var surnames = readCensusFile("us_census_2000_surnames.txt")
    var maleNames = readCensusFile("us_census_2000_male_first.txt")
    var femaleNames = readCensusFile("us_census_2000_female_first.txt")

    val allDicts = Set(passwords, english, surnames, maleNames, femaleNames)

    passwords = filterDup(passwords, allDicts -- Set(passwords))
    english = filterDup(english, allDicts -- Set(english))
    surnames = filterDup(surnames, allDicts -- Set(surnames))
    maleNames = filterDup(maleNames, allDicts -- Set(maleNames))
    femaleNames = filterDup(femaleNames, allDicts -- Set(femaleNames))

    (passwords, surnames, maleNames, femaleNames, english)
  }

  // Ranked wordlists
  lazy val passwords = dicts._1
  lazy val surnames = dicts._2
  lazy val maleNames = dicts._3
  lazy val femaleNames = dicts._4
  lazy val english = dicts._5

  def filterDup(list: List[String], lists: Set[List[String]]): List[String] = {
    val maxRank = list.length
    val dict = rankedDict(list)
    val dicts = lists map (rankedDict(_))

    list.filter((w) => dict(w) < dicts.foldLeft(maxRank)((i, d) => min(d.getOrElse(w,i), i)))
  }

  private def rankedDict(list: List[String]) = list.zipWithIndex.map(t => (t._1, t._2 + 1)).toMap

  private def readWordList(file: String) = readLines(file).map(_.trim.toLowerCase).toList

  private def readCensusFile(file: String) = readLines(file).map(_.split("\\W")(0).toLowerCase).toList

  private def readLines(file: String) = Option(Thread.currentThread().getContextClassLoader.getResourceAsStream(file)) match {
    case Some(stream) => io.Source.fromInputStream(stream).getLines()
    case None => throw new IllegalArgumentException("Failed to load classpath resource " + file)
  }
}