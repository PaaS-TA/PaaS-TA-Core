package core

import "fmt"

func AZs(numberOfAZs int) []string {
	azs := []string{}
	for i := 1; i <= numberOfAZs; i++ {
		azs = append(azs, fmt.Sprintf("z%d", i))
	}
	return azs
}
