package fakes

type Logger struct {
	PrintlnCall struct {
		CallCount int
		Receives  struct {
			Values []interface{}
		}
		AllValues [][]interface{}
	}
}

func (l *Logger) Println(v ...interface{}) {
	l.PrintlnCall.Receives.Values = v
	l.PrintlnCall.AllValues = append(l.PrintlnCall.AllValues, v)
}
