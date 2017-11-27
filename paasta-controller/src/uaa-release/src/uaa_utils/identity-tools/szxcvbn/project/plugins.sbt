resolvers += "Local Maven Repository" at "file://"+Path.userHome+"/.m2/repository"

addSbtPlugin("net.virtual-void" % "sbt-dependency-graph" % "0.6.0")
