package vars

import "strings"

type StringList map[string]struct{}

func (sl StringList) Set(arg string) error {
	sl[arg] = struct{}{}
	return nil
}

func (sl StringList) String() string {
	return strings.Join(sl.Values(), ",")
}

func (sl StringList) Get() interface{} {
	return sl.Values()
}

func (sl StringList) Values() []string {
	var result []string
	for k, _ := range sl {
		result = append(result, k)
	}
	return result
}
