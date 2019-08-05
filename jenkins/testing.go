package jenkins

import "context"

type StubEventTracer struct {
	ReturnValue error
	calls       int
}

func (s *StubEventTracer) handleBuild(ctx context.Context, be *BuildEvent) error {
	s.calls++
	return s.ReturnValue
}
