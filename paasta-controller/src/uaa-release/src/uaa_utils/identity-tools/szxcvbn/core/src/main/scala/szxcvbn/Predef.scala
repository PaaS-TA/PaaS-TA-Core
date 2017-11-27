package szxcvbn

import java.lang.Character.{isLowerCase, isDigit, isUpperCase}

object Predef {
//  type Password = IndexedSeq[Char]

  private val Log2 = math.log(2)

  def log2(n: Double) = math.log(n) / Log2

  val Log2of10 = log2(10)
  val Log2of26 = log2(26)

  def bruteForceCardinality(password: String) = {
    var lower, upper, digits, symbols = 0

    password.foreach((c) => {
      if (isLowerCase(c)) {
        lower = 26
      } else if (isDigit(c)) {
        digits = 10
      } else if (isUpperCase(c)) {
        upper = 26
      } else {
        symbols = 33
      }
    })

    lower + upper + digits + symbols
  }

  val Minute = 60
  val Hour = 60 * Minute
  val Day = 24 * Hour
  val Month = 31 * Day
  val Year = 12 * Month
  val Century = 100L * Year

  def displayTime(seconds: Double) =
    if (seconds < Minute)
      "instant"
    else if (seconds < Hour)
      "%.2f minutes".format(seconds/Minute)
    else if (seconds < Day)
      "%.2f hours".format(seconds/Hour)
    else if (seconds < Month)
      "%.2f days".format(seconds/Day)
    else if (seconds < Year)
      "%.2f months".format(seconds/Month)
    else if (seconds < Century)
      "%.2f years".format(seconds/Year)
    else "centuries"


  def nCk (n: Int, k: Int): Int = {
    if (k == 0)
      return 1
    if (k > n)
      return 0

    var r = 1
    var m = n

    for (d <- 1 to k) {
      r *= m
      r /= d
      m -= 1
    }
    r
  }

  /**
   * Matched sequences must be longer than two chars
   */
  def okLength(from: Int, to: Int) = (to - from) >= 2
}
