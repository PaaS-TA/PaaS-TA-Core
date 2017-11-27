
import org.scalatest.{GivenWhenThen, FunSpec}

import szxcvbn._

class MatchingSpec extends FunSpec with GivenWhenThen {

  describe("A dictionary matcher") {

    it ("should return a match for each matched substring in a word") {

      given("""a matcher with dictionary containing "password", "password1" and "word"""")
      val m = DictMatcher("test", Seq("password", "password1", "hello", "word", "dorw"))

      when("it tests the string 'password1'")
      val matches = m.matches("password1")

      then("it should return three matches")
      assert(matches.size === 3)

      and("the matchedWord for each should be the word")
      assert(matches(0).matchedWord === "word")

      and("the token for each should be the word")
      assert(matches(0).token === "word")

      and("the rank of each match should be the (unit-offset) dictionary rank")
      assert(matches(0).rank === 4)

      and("the start and end indices of each match should be the character indices in the password")
      assert(matches(0).start === 4)
      assert(matches(0).end === 7)

      and("the entropy for each match should be correct")

    }
  }

  describe ("A sequence matcher") {

    it ("should match all sequences of length 3 or greater") {

      given("a sequence matcher for standard sequences")
      val m = SequenceMatcher(Zxcvbn.StandardSequences)

      when("it tests the string 'ab34567abcdehgfVWXYZ'")
      val matches = m.matches("ab34567abcdehgfVWXYZ")

      then("it should match 4 sequences")
      assert(matches.size === 4)

      and("the token should be the subsequence of the original string")
      assert(matches(3).token === "VWXYZ")
      assert(matches(2).token === "hgf")
      assert(matches(1).token === "abcde")
      assert(matches(0).token === "34567")

      and("the start and end indices of each match should be the character indices in the original string")
      assert(matches(3).start === 15)
      assert(matches(3).end === 19)
      assert(matches(2).start === 12)
      assert(matches(2).end === 14)

      and("the direction should be descending for 'hgf'")
      assert(matches(2).ascending === false)
    }
  }


  describe ("A repeat matcher") {

    it ("should match all repeats of length 3 or greater") {

      given("a repeat matcher")
      val m = RepeatMatcher

      when("it tests the string 'aaYYY****s0000你你你好")
      val matches = m.matches("aaYYY****s0000你你你好")

      then("it should find 4 repeats")
      assert(matches.size === 4)

      and("the token should be the subsequence of the original string")
      assert(matches(0).token === "YYY")
      assert(matches(3).token === "你你你")

      and("the start and end indices of each match should be the character indices in the original string")
      assert(matches(0).start === 2)
      assert(matches(0).end === 4)
      assert(matches(1).start === 5)
      assert(matches(1).end === 8)
      assert(matches(3).start === 14)
      assert(matches(3).end === 16)
    }
  }

  describe ("A l33t matcher") {

    it ("should find a match with a l33t substitution") {

      given("a l33t matcher")
      val m = new L33tMatcher(List(DictMatcher("l33tdict", Seq("abacus","solution", "absolution"))))

      when("it tests the string '4b4cu$'")
      var matches = m.matches("4b4cu$")

      then("it should match 'abacus'")
      assert(matches.size === 1)
      assert(matches(0).unl33ted === "abacus")

      when("it tests the string '48$0|u+10n'")
      matches = m.matches("48$0|u+10n")

      then("it should match 'solution' and 'absolution'")
      assert(matches.size === 2)
      assert(matches(0).unl33ted === "solution")

    }
  }

  describe ("A spatial matcher") {

    it ("should match spatial sequences of length 3 or greater") {

      given("a 'qwerty' spatial matcher")
      val m = SpatialMatcher(Data.adjacencyGraphs(0))

      when("it tests the string 'qwzxcvbnok'")
      var matches = m.matches("qwzxcvbnok")

      then("it should find one match")
      assert(matches.size === 1)

      and("the token should be 'zxcvbn'")
      assert(matches(0).token === "zxcvbn")

      and("it should have one turn")
      assert(matches(0).turns === 1)

      when("it tests the string 'szxcvbn0zxcvbn'")
      matches = m.matches("szxcvbn0zxcvbn")

      then("it should find 2 matches, 'szxcvbn' and 'zxcvbn'")
      assert(matches.size === 2)
      assert(matches(0).token === "szxcvbn")
      assert(matches(1).token === "zxcvbn")
    }

    it ("should match sequences containing multiple turns") {
      given("a 'qwerty' spatial matcher")

      val m = SpatialMatcher(Data.adjacencyGraphs(0))

      when("it tests the string 'dfghyt56'")
      var matches = m.matches("dfghyt56")

      then("it should find one match, 'dfghyt56' with 5 turns")
      assert(matches.size === 1)
      assert(matches(0).token === "dfghyt56")
      assert(matches(0).turns === 5)
    }
  }

  describe ("A digits matcher") {

    it ("should match digit sequences of 3 or more characters") {
      given("a digits matcher")

      when("it tests the string 'abc20176hj984182a3'")
      var matches = DigitsMatcher.matches("abc20176hj984182a3")

      then("it should find two matches")
      assert(matches.size === 2)
      assert(matches(0).token === "20176")
      assert(matches(0).start === 3)
      assert(matches(0).end === 7)
    }

    it ("should match 4 digit years") {
      given("a digits matcher")

      when("it tests the string 'abc1988g2012'")
      var matches = DigitsMatcher.matches("abc1988g2012")

      then("it should find two year matches")
      assert(matches.size === 2)
      matches(0) match {
        case YearMatch(_, _, year) => assert(year === "1988")
        case _ => fail("Not a year")
      }
      matches(1) match {
        case YearMatch(_, _, year) => assert(year === "2012")
        case _ => fail("Not a year")
      }
    }

    it ("should match 4 digit dates") {
      given("a digits matcher")

      when("it tests the string 'abc8821_0405_9401_1009'")
      var matches = DigitsMatcher.matches("abc8821_0405_9401_1009")

      then("it should find 4 matches")
      assert(matches.size === 4)

      and("two of them should be date matches")
      matches(0) match {
        case DateMatch(_, _, t, year, _) =>
          assert(year === 21)
          assert(t === "8821")
        case _ => fail("Not a date")
      }

      matches(2) match {
        case DateMatch(_, _, t, year, _) =>
          assert(year === 1)
          assert(t === "9401")
        case _ => fail("Not a date")
      }

      assert(matches(1).isInstanceOf[DigitsMatch])
      assert(matches(3).isInstanceOf[DigitsMatch])
    }

    it ("should match 5 digit dates") {
      given("a digits matcher")

      when("it tests the string 'abc23459_01008_76932_31120'")
      var matches = DigitsMatcher.matches("abc23459_01008_76932_31120")

      then("it should find 4 matches")
      assert(matches.size === 4)

      and("'23459' should be a date match")
      matches(0) match {
        case DateMatch(_, _, t, year, _) =>
          assert(year === 59)
          assert(t === "23459")
        case _ => fail("Not a date")
      }

      and("'31120' should be a date match")
      matches(3) match {
        case DateMatch(_, _, t, year, _) =>
          assert(year === 20)
          assert(t === "31120")
        case _ => fail("Not a date")
      }

      and("the others should be digit matches")
      assert(matches(1).isInstanceOf[DigitsMatch])
      assert(matches(2).isInstanceOf[DigitsMatch])
    }

    it ("should match 6 digit dates") {
      given("a digits matcher")

      when("it tests the string 'abc200159_01008_76932_001231'")
      var matches = DigitsMatcher.matches("abc200159_597099_76932_001231")

      then("it should find 4 matches")
      assert(matches.size === 4)

      and("'592001' should be a date")
      matches(0) match {
        case DateMatch(_, _, t, year, _) =>
          assert(year === 59)
          assert(t === "200159")
        case _ => fail("Not a date")
      }

      and("'001231' should be a date")
      matches(3) match {
        case DateMatch(_, _, t, year, _) =>
          assert(year === 0)
          assert(t === "001231")
        case _ => fail("Not a date")
      }

      and("the others should be digit matches")
      assert(matches(1).isInstanceOf[DigitsMatch])
      assert(matches(2).isInstanceOf[DigitsMatch])
    }

    it ("should match 8 digit dates") {
      given("a digits matcher")

      when("it tests the string 'a20011509_19051947_76322304_00012001'")
      var matches = DigitsMatcher.matches("a20011509_19051947_76322304_00012001")

      then("it should find 4 matches")
      assert(matches.size === 4)

      and("'20011509' should be a date")
      matches(0) match {
        case DateMatch(_, _, t, year, _) =>
          assert(year === 2001)
          assert(t === "20011509")
        case _ => fail("Not a date")
      }

      and("'19051947' should be a date")
      matches(1) match {
        case DateMatch(_, _, t, year, _) =>
          assert(year === 1947)
          assert(t === "19051947")
        case _ => fail("Not a date")
      }

      and("the others should be digit matches")
      assert(matches(2).isInstanceOf[DigitsMatch])
      assert(matches(3).isInstanceOf[DigitsMatch])
    }
  }

}
