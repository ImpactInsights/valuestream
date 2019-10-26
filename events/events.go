package events

type SpanState string

const (
	StartState        SpanState = "start"
	EndState          SpanState = "end"
	IntermediaryState           = "intermediary"
	UnknownState                = "unknown"
)
