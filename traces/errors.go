package traces

type SpanMissingIDError struct {
	Err error
}

func (s SpanMissingIDError) Error() string {
	return s.Err.Error()
}

type SpanMissingError struct {
	Err error
}

func (s SpanMissingError) Error() string {
	return s.Err.Error()
}
