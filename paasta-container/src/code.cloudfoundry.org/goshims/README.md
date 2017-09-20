### osshim

an interface for faking out your os, just in case your code interacts with the file system heavily and you want to be able to induce failures.

batteries included!
the Os implementation in the base directory calls through to go's os package,
the Os implementation in the fakes directory calls to a counterfeiter fake for use in test.

we generated this using a crazy, experimental fork of maxbrunsfeld/counterfeiter you can find [here](https://github.com/cwlbraa/counterfeiter).
