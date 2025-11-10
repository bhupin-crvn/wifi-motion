package sse

type FlowError struct {
	Err  error
	Next bool
}

func (e *FlowError) Error() string {
	return e.Err.Error()
}

func NewFlowError(err error, next bool) *FlowError {
	return &FlowError{
		Err:  err,
		Next: next,
	}
}
