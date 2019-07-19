package traces

import "fmt"

func PrefixSCM(s string) string {
	return fmt.Sprintf("SCM-%s", s)
}

func PrefixISSUE(s string) string {
	return fmt.Sprintf("ISSUE-%s", s)
}
