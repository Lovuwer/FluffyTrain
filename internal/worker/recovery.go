// package worker - job processing worker
package worker

import (
	"fmt"
	"log/slog"
	"runtime/debug"
)

// RecoverResult holds information about a recovered panic.
type RecoverResult struct {
	Recovered bool
	Error     error
	Stack     string
}

// RecoverPanic recovers from a panic and returns information about it.
func RecoverPanic(logger *slog.Logger, jobID, jobType string) *RecoverResult {
	if r := recover(); r != nil {
		stack := string(debug.Stack())
		
		var err error
		switch v := r.(type) {
		case error:
			err = v
		case string:
			err = fmt.Errorf("%s", v)
		default:
			err = fmt.Errorf("%v", v)
		}

		if logger != nil {
			logger.Error("job panic recovered",
				"job_id", jobID,
				"job_type", jobType,
				"panic", r,
				"stack", stack,
			)
		}

		return &RecoverResult{
			Recovered: true,
			Error:     fmt.Errorf("panic: %w", err),
			Stack:     stack,
		}
	}

	return &RecoverResult{
		Recovered: false,
	}
}
