package szxcvbn

import szxcvbn.Predef._
import Character._

/**
 * Stuff for entropy calculations
 */
object Entropy {

  def extraUpperCaseEntropy(word: String): Double = {
    val (nU,nL) = countUpperLowerCase(word)

    nU match {
      case 0 => 0
      // Simple capitalization Blah, blaH
      // a capitalized word is the most common capitalization scheme, so it only doubles the search space
      // (uncapitalized + capitalized): 1 extra bit of entropy.
      // all-caps and end-capitalized are common enough too. Underestimate as 1 extra bit to be safe.

      case 1 if (isUpperCase(word(0)) || isUpperCase(word(word.length-1))) => 1

      // otherwise calculate the number of ways to capitalize U+L uppercase+lowercase letters with U uppercase letters
      // or less. Or, if there's more uppercase than lower (for e.g. PASSwORD), the number of ways to lowercase U+L
      // letters with L lowercase letters
      case _ =>
        if (nL == 0) 1
        else {
          val possibilities: Int = Range.inclusive(0,  math.min(nL,nU)).foldLeft(0)(_ + nCk(nU + nL, _))
          log2(possibilities)
        }
    }
  }

  def countUpperLowerCase(word: String): (Int,Int) = countBack(word, word.length-1, (0,0))

  private def countBack(word: String, pos: Int, acc: (Int,Int)): (Int,Int) =
    if (pos == 0) incrUL(word(pos), acc)
    else countBack(word, pos - 1, incrUL(word(pos), acc))

  private def incrUL(c: Char, acc: (Int,Int)): (Int,Int) =
    if (isLowerCase(c)) (acc._1, acc._2 + 1)
    else if (isUpperCase(c)) (acc._1 + 1, acc._2)
    else acc

  def extraL33tEntropy(word: String, subs: List[(Char,Char)]): Double = {
    val possibilities = subs.foldLeft(0)((total,sub) => {
        val (nS,nU) = word.foldLeft((0,0))((acc, c) => c match {
          case sub._1 => (acc._1 + 1, acc._2)
          case sub._2 => (acc._1, acc._2 + 1)
          case _      => acc
        })
        total + Range.inclusive(0,  math.min(nS,nU)).foldLeft(0)(_ + nCk(nU + nS, _))
      })

    possibilities match {
      case 1 => 1.0
      case p => log2(p)
    }
  }
}
