package szxcvbn

import scala.math._
import szxcvbn.Predef._
import Entropy._
import scala.util.matching.Regex

sealed trait Match {
  /**
   * The character index where the match starts in a password
   */
  def start: Int

  /**
   * The character index where the match ends
   */
  def end: Int

  /**
   * The matched token, a substring of the password.
   */
  def token: String

  /**
   * Entropy of the match
   */
  def entropy: Double

  /**
   * The type of the match
   */
  def pattern: String

  def <(m: Match) = (start - m.start) match {
    case 0 => end - m.end < 0
    case x => x < 0
  }
}

final case class DictMatch(start: Int, end: Int, token: String, dictName: String, matchedWord: String, rank: Int) extends Match {
  val pattern = "dictionary"
  val entropy = log2(rank) + extraUpperCaseEntropy(token)
}

final case class L33tMatch(start: Int, end: Int, token: String, dictName: String, unl33ted: String, rank: Int, subs: List[(Char,Char)]) extends Match {
  val pattern = "dictionary"

  val baseEntropy = log2(rank) + extraUpperCaseEntropy(token)
  val entropy = baseEntropy + extraL33tEntropy(token, subs)
}

final case class SequenceMatch(start: Int, end: Int, token: String, name: String, space: Int, ascending: Boolean) extends Match {
  val pattern = "sequence"
  val entropy = baseEntropy(token(0)) + log2(token.length) + (if (ascending) 0 else 1)

  private def baseEntropy(startChar: Char) = {
    if (startChar == '1' || startChar == 'a') 1.0
    else if (Character.isDigit(startChar)) Log2of10
    else if (Character.isLetter(startChar)) Log2of26
    else Log2of26 + 1
  }
}

final case class RepeatMatch(start: Int, end: Int, token: String, repeatedChar: Char) extends Match {
  val pattern = "repeat"
  val entropy = log2(bruteForceCardinality(token) * token.length)
}

final case class BruteForceMatch(start: Int, end: Int, token: String, cardinality: Int) extends Match {
  val pattern = "bruteforce"
  val entropy = log2(pow(cardinality, end - start + 1))
}

final case class SpatialMatch(start: Int, end: Int, token: String, name: String, entropy: Double, turns: Int, shiftedCount: Int) extends Match {
  val pattern = "spatial"
}

final case class DigitsMatch(start: Int, end: Int, token: String) extends Match {
  val pattern = "digits"
  val entropy = log2(math.pow(10, token.length))
}

final case class YearMatch(start: Int, end: Int, token: String) extends Match {
  private val NumYears = 119 // years match against 1900 - 2019

  val pattern = "year"
  val entropy = log2(NumYears)
}

final case class DateMatch(start: Int, end: Int, token: String, year: Int, separator: Boolean = false) extends Match {
  private val TwoDigitYearEntropy = log2(31 * 12 * 100)
  private val FourDigitYearEntropy = log2(31 * 12 * 119)

  val pattern = "date"
  val entropy = (if (year < 100) TwoDigitYearEntropy else FourDigitYearEntropy) + (if (separator) 2 else 0)
}

trait Matcher[+A <: Match] {
  def matches(password: String): List[A]
}

final case class DictMatcher(name: String, wordList: Seq[String]) extends Matcher[DictMatch] {
  private val dict = wordList.zipWithIndex.map(t => (t._1, t._2 + 1)).toMap

  def matches(password: String) = {
    val lcPass = password.toLowerCase
    var matches: List[DictMatch] = Nil

    for (i <- 0 until lcPass.length)
      for (j <- i until lcPass.length) {
        val word = lcPass.substring(i, j + 1)
        dict.get(word) match {
          case Some(rank) =>
            matches = DictMatch(i, j, password.substring(i, j + 1), name, word, rank) :: matches
          case None =>
        }
      }

    matches
  }
}

object RepeatMatcher extends Matcher[RepeatMatch] {
  def matches(password: String) = findMatches(password, password.length-1)

  private def findMatches(passwd: String, to: Int, matches: List[RepeatMatch] = Nil): List[RepeatMatch] =
    if (to > 0) {
      val from = extendRepeat(passwd(to), passwd, to)

      findMatches(passwd,
                  from - 1,
                  if (okLength(from, to))
                    RepeatMatch(from, to, passwd.substring(from, to+1), passwd(to)) :: matches
                  else
                    matches)
    } else matches


  private def extendRepeat(c: Char, word: String, pos: Int): Int =
    if (pos > 0 && word(pos-1) == c) extendRepeat(c, word, pos-1) else pos
}

final case class SequenceMatcher(namedSequences: List[(String,String)]) extends Matcher[SequenceMatch] {
  def matches(passwd: String) = findMatches(passwd, passwd.length - 1)

  private def findMatches(passwd: String, to: Int, matches: List[SequenceMatch] = Nil): List[SequenceMatch] =
    if (to > 0)
      findCandidateSequence(passwd(to-1), passwd(to), namedSequences) match {
        case Some((name, sequence, direction)) =>
          // We have a valid sequence, now see how far it goes
          val from = extendSequence(to - 1, passwd, sequence, direction)

          findMatches(passwd,
                      from - 1,
                      if (okLength(from, to))
                        SequenceMatch(from, to, passwd.substring(from,to+1), name, sequence.length, direction == 1) :: matches
                      else
                        matches)
        case None =>
          findMatches(passwd, to - 1, matches)
      }
    else matches

  /**
   * Find the maximum extent of the sequence substring in the word, working back from the given position.
   */
  private def extendSequence(pos: Int, passwd: String, seq: String, direction: Int): Int = {
    if (pos > 0)
      sequenceMatch(passwd(pos-1), passwd(pos), seq) match {
        // Is this a continuation of the sequence
        case Some(d) if (d == direction) =>
          // pos-1 is in the sequence, so extend from there
          extendSequence(pos - 1, passwd, seq, direction)
        case _ =>
          pos
      }
    else 0
  }

  /**
   * Given a pair of adjoining characters, find a sequence which contains them in either order.
   */
  private def findCandidateSequence(c1: Char, c2: Char, sequences: List[(String, String)]): Option[(String,String,Int)] =
    sequences match {
      case s :: ss =>
        sequenceMatch(c1, c2, s._2) match {
          case Some(direction) =>
            Some(s._1, s._2, direction)
          case _ =>
            findCandidateSequence(c1, c2, ss)
        }
      case Nil => None
    }

  /**
   * Determines whether the two characters are adjacent in a given sequence, returning an optional direction
   * value if they are. Direction is 1 if c1,c2 are in the sequence order, -1 if they are in reverse order.
   */
  private def sequenceMatch(c1: Char, c2: Char, seq: String): Option[Int] = {
    val (i,j) = (seq.indexOf(c1), seq.indexOf(c2))

    if (i >= 0 && j >= 0 && abs(j - i) == 1) Some(j - i) else None
  }
}

final case class SpatialMatcher(graph: AdjacencyGraph) extends Matcher[SpatialMatch] {

  def matches(passwd: String) = findMatches(passwd, passwd.length - 1)

  private def findMatches(passwd: String, to: Int, matches: List[SpatialMatch] = Nil): List[SpatialMatch] =
    if (to > 0) {
      val (from, turns, nShifted) = walkAdjacents(passwd, to)

      findMatches(passwd,
                  from - 1,
                  if (okLength(from, to)) {
                    // Walk doesn't check whether start pos is shifted
                    val totalShifted = if (graph.adjacentMatch(passwd(to-1),passwd(to)).get._2) nShifted + 1  else nShifted
                    val token = passwd.slice(from, to+1)
                    SpatialMatch(from, to, token, graph.name, graph.entropy(token.length, turns, totalShifted), turns, totalShifted) :: matches
                  }
                  else matches)

    }
    else matches

  /**
   * Walk back from the current position and continue while an adjacency match is found for the current character.
   */
  private def walkAdjacents(password: String, pos: Int, currentDirection: Int = -Int.MaxValue, turns: Int = 0, shiftedCount: Int = 0): (Int, Int, Int) =
    if (pos > 0)
      graph.adjacentMatch(password(pos), password(pos-1)) match {
        case None =>
          (pos, turns, shiftedCount)
        case Some((direction, shifted)) =>
          walkAdjacents(password, pos - 1, direction, turns + (if (direction != currentDirection) 1 else 0), shiftedCount + (if (shifted) 1 else 0))
      }
    else (pos, turns, shiftedCount)
}

final case class L33tMatcher(matchers: List[DictMatcher]) extends Matcher[L33tMatch]  {

  val L33tTable = Map(
    'a' -> Seq('4', '@'),
    'b' -> Seq('8'),
    'c' -> Seq('(', '{', '[', '<'),
    'e' -> Seq('3'),
    'g' -> Seq('6', '9'),
    'i' -> Seq('1', '!', '|'),
    'l' -> Seq('1', '|', '7'),
    'o' -> Seq('0'),
    's' -> Seq('$', '5'),
    't' -> Seq('+', '7'),
    'x' -> Seq('%'),
    'z' -> Seq('2'))

  val L33tSubs: Map[Char,Seq[Char]] = {
    val k = L33tTable.values.flatten.toList.distinct
    val v = k map { e => L33tTable.keys.toList.filter {L33tTable(_).contains(e)} }
    (k zip v).toMap
  }

  def matches(password: String): List[L33tMatch] = {
    val subbed: StringBuilder = new StringBuilder
    var multiSubs: List[(Char, Seq[Char])] = Nil
    // Actual substitutions used
    var subs = collection.mutable.Set[(Char,Char)]()

    password.foreach(c =>
      L33tSubs.get(c) match {
        case None =>
          subbed.append(c)
        case Some(seq) => seq match {
          // Single char sub. Do the substitution
          case Seq(sub) =>
            subbed.append(sub)
            subs += ((c, sub))
          case multiSub =>
            // Leave the unsubbed char. We need to create multiple strings
            subbed.append(c)
            multiSub.foreach(sub => subs += ((c, sub)))
            multiSubs ::= (c, multiSub)
        }
      }
    )

    if (subs.isEmpty) {
      return Nil
    }

    val words = multiSubs match {
      case Nil => List(subbed)
      case s   => doMultiSubs(List(subbed), s)
    }

    val dictMatches = words.flatMap(w => matchers.flatMap(_.matches(w.toString())))

    for {
      dm <- dictMatches
      token = password.substring(dm.start, dm.end+1)
      // Does the match actually contain any substitutions?
      if (token.toLowerCase != dm.matchedWord)
    } yield {
      L33tMatch(dm.start, dm.end, token, dm.dictName, dm.matchedWord, dm.rank, subs.filter(isUsedSub(token, dm.matchedWord, _)).toList)
    }
  }

  private def isUsedSub(token: String, matchedWord: String, sub: (Char,Char)) = {
    val i = token.indexOf(sub._1)
    i >= 0 && matchedWord(i) == sub._2
  }

  private def doMultiSubs(targets: List[StringBuilder], multiSubs: List[(Char, Seq[Char])]): List[StringBuilder] = {
    multiSubs match {
      case Seq((c,s)) =>
        targets.flatMap(t => doMultiSub(t, c, s))
      case seq =>
        doMultiSubs(targets.flatMap(doMultiSub(_, seq.head._1, seq.head._2)), multiSubs.tail)
    }
  }

  /**
   * Performs each one of multiple substitutions for a single character on a StringBuilder and
   * returns a copy for each substitution.
   */
  private def doMultiSub(target: StringBuilder, c: Char, substitutions: Seq[Char]): Seq[StringBuilder] =
    substitutions.map(s => singleSub(target.clone(), c, s))

  private def singleSub(sb: StringBuilder, c: Char, sub: Char): StringBuilder = {
    for (i <- 1 until sb.length) {
      if (sb(i) == c)
        sb.setCharAt(i, sub)
    }
    sb
  }

}

object DateChecker {
  private val Exp10 = Seq(1,10,100,1000,10000,100000,1000000)

  def valid2DigitYearDate(n: Int, l: Int): Option[Int] = {
    if (l > 6)
      None
    else {
      if (validDm(n / 100, l-2)) Some(n % 100) // year suffix
      else if (validDm(n % Exp10(l-2), l-2)) Some(n / Exp10(l-2)) // year prefix
      else None
    }
  }

  def valid4DigitYearDate(n: Int, l: Int): Option[Int] = {
    if (l < 6 || l > 8) None
    else {
      if (validDm(n / 10000, l-4) && valid4DgtYr(n % 10000)) Some(n % 10000)
      else if (validDm(n % Exp10(l-4), l-4) &&  valid4DgtYr(n / Exp10(l-4))) Some(n / Exp10(l-4))
      else None
    }
  }

  def valid4DgtYr(y: Int) = y >= 1900 && y < 2020

  def valid2DgtYr(y: Int) = y < 100

  def validDm(dm: Int, length: Int): Boolean =
    length match {
      case 2 => validDayMonth(dm/10, dm % 10)
      case 3 => validDayMonth(dm/10, dm % 10) || validDayMonth(dm/100, dm % 100)
      case 4 => validDayMonth(dm/100, dm % 100)
      case _ => false
    }

  def validDayMonth(day: Int, month: Int): Boolean =
    (month <= 12 && day <= 31 || day <= 12 && month <= 31) && day * month > 0
}

import DateChecker._

/**
 * Matches sequences of 3 or more digits.
 *
 * Those which are valid dates or years will be returned as specific match types with lower entropy.
 */
object DigitsMatcher extends Matcher[Match] {
  private val Digits = "\\d{3,}".r

  def matches(password: String) = Digits.findAllIn(password).matchData.map {
    m => doMatch(m.start, m.end-1, m.matched)
  }.toList

  def doMatch(start: Int, end: Int, token: String): Match =
    (if (token.length >= 4 && token.length <= 8) {
      // Check whether it can be a date
      val n = token.toInt

      if (token.length == 4 && valid4DgtYr(n))
        Some(YearMatch(start, end, token))
      else
        valid2DigitYearDate(n, token.length) orElse valid4DigitYearDate(n, token.length) map { DateMatch(start, end, token, _) }
    } else None).getOrElse(DigitsMatch(start, end, token))
}

/**
 * Matches dates with separators
 */
object DateMatcher extends Matcher[DateMatch] {
  val DateYearSuffix = """(\d{1,2})(\s|-|/|\\|\.)(\d{1,2})\2(19\d{2}|200\d|201\d|\d{2})""".r
  val DateYearPrefix = """(19\d{2}|200\d|201\d|\d{2})(\s|-|/|\\|\.)(\d{1,2})\2(\d{1,2})""".r

  def matches(password: String) = {
    val matches = (DateYearSuffix.findAllIn(password).matchData ++
      DateYearSuffix.findAllIn(password).matchData).toSeq
    val dates = matches.collect(validDate).toList
    dates
  }

  private val validDate: PartialFunction[Regex.Match, DateMatch] = {
    case ValidDate(d)  => d
  }

  private object ValidDate {
      def unapply(m: Regex.Match): Option[DateMatch] = {
        val (day,month,year) = (m.group(1).toInt, m.group(3).toInt, m.group(4).toInt)
        if ((valid4DgtYr(year) || valid2DgtYr(year)) && validDayMonth(day, month))
          Some(DateMatch(m.start, m.end-1, m.matched, year, true))
        else if ((valid4DgtYr(day) || valid2DgtYr(day)) && validDayMonth(year, month))
          Some(DateMatch(m.start, m.end-1, m.matched, day, true))
        else None
      }
  }
}
