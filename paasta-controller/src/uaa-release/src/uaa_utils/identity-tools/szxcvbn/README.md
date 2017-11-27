A Scala password quality checker, based on Dan Wheeler's [zxcvbn](https://github.com/lowe/zxcvbn).
Dan's [blog article](http://tech.dropbox.com/?p=165) gives a good overview of the model and the algorithms used.

The main code is in the *core* subproject. The *server* project contains a simple
[unfiltered](https://github.com/unfiltered/unfiltered) server which serves up
JSON data in response to submitted password values. It can be tested using the
URL [http://localhost:8080/password.html](http://localhost:8080/password.html).

Use [sbt](https://github.com/harrah/xsbt) 0.12 to build and run.
