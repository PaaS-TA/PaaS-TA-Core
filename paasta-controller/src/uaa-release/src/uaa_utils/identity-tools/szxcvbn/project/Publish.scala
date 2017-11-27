import sbt._
import Keys._

/**
 * Settings for publishing to Sonatype OSS
 */
object Publish {

  val settings = Seq(
    publishMavenStyle := true,
    publishArtifact in Test := false,
    pomIncludeRepository := { _ => false },

    publishTo <<= version { v: String =>
      val nexus = "https://oss.sonatype.org/"
      if (v.trim.endsWith("SNAPSHOT")) Some("snapshots" at nexus + "content/repositories/snapshots")
      else Some("releases" at nexus + "service/local/staging/deploy/maven2")
    },

    pomExtra := (
      <url>https://github.com/tekul/szxcvbn</url>
      <licenses>
        <license>
          <name>MIT</name>
          <url>http://www.opensource.org/licenses/MIT</url>
          <distribution>repo</distribution>
        </license>
      </licenses>
      <scm>
        <url>git@github.com:tekul/szxcvbn.git</url>
        <connection>scm:git:git@github.com:tekul/szxcvbn.git</connection>
      </scm>
      <developers>
        <developer>
          <id>tekul</id>
          <name>Luke Taylor</name>
          <url>https://github.com/tekul</url>
        </developer>
      </developers>
    )
  )



}