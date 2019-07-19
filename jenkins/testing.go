package jenkins

type StubEventTracer struct {
	ReturnValue error
	calls       int
}

func (s *StubEventTracer) handleBuild(be *BuildEvent) error {
	s.calls++
	return s.ReturnValue
}
