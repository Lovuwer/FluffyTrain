// Package handlers provides example job handlers for testing.
package handlers

import (
	"github.com/joek3softwares-boop/titan/internal/worker"
)

// RegisterAll registers all example handlers with the registry.
func RegisterAll(registry *worker.Registry) {
	registry.RegisterFunc("echo", Echo)
	registry.RegisterFunc("sleep", Sleep)
	registry.RegisterFunc("fail_always", FailAlways)
	registry.RegisterFunc("fail_n_times", FailNTimes)
	registry.RegisterFunc("panic", Panic)
	registry.RegisterFunc("divide_by_zero", DivideByZero)
	registry.RegisterFunc("http_request", HTTPRequest)
	registry.RegisterFunc("email_mock", EmailMock)
}
