import org.scalatest.FunSuite
import org.scalatest.matchers.ShouldMatchers
import szxcvbn.Data

class AdjacencyGraphSuite extends FunSuite with ShouldMatchers {

  test("An adjacency graph should return the correct entropy for (length,turn) combinations") {
    val qwerty = Data.adjacencyGraphs(0)

    qwerty.entropy(5,3,0) should be(16.634 plusOrMinus(0.005))
  }
}
