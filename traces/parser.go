package traces

import "regexp"

func Matches(body string) ([]string, error) {
	r, err := regexp.Compile(
		"vstrace-[0-9A-Za-z]+-[0-9A-Za-z_]+-[0-9A-Za-z-]+-[0-9A-Za-z-]+",
	)
	if err != nil {
		return nil, err
	}

	matches := r.FindStringSubmatch(body)
	return matches, nil
}
