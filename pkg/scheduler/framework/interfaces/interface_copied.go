// This file was copied from k8s.io/kubernetes/pkg/scheduler/framework/interfaces.go and modified

package interfaces

import (
	"errors"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// Code is the Status code/type which is returned from plugins.
type Code int

// These are predefined codes used in a Status.
const (
	// Success means that plugin ran correctly and found subscription schedulable.
	// NOTE: A nil status is also considered as "Success".
	Success Code = iota
	// Error is used for internal plugin errors, unexpected input, etc.
	Error
	// Unschedulable is used when a plugin finds a subscription unschedulable. The scheduler might attempt to
	// preempt other subscriptions to get this subscription scheduled. Use UnschedulableAndUnresolvable to make the
	// scheduler skip preemption.
	// The accompanying status message should explain why the subscription is unschedulable.
	Unschedulable
	// UnschedulableAndUnresolvable is used when a plugin finds a subscription unschedulable and
	// preemption would not change anything. Plugins should return Unschedulable if it is possible
	// that the subscription can get scheduled with preemption.
	// The accompanying status message should explain why the subscription is unschedulable.
	UnschedulableAndUnresolvable
	// Wait is used when a Permit plugin finds a subscription scheduling should wait.
	Wait
	// Skip is used when a Bind plugin chooses to skip binding.
	Skip
)

// This list should be exactly the same as the codes iota defined above in the same order.
var codes = []string{"Success", "Error", "Unschedulable", "UnschedulableAndUnresolvable", "Wait", "Skip"}

func (c Code) String() string {
	return codes[c]
}

// Status indicates the result of running a plugin. It consists of a code, a
// message, (optionally) an error and an plugin name it fails by. When the status
// code is not `Success`, the reasons should explain why.
// NOTE: A nil Status is also considered as Success.
// Copied from k8s.io/kubernetes/pkg/scheduler/framework/interface.go and modified
type Status struct {
	code    Code
	reasons []string
	err     error
	// failedPlugin is an optional field that records the plugin name a Description failed by.
	// It's set by the framework when code is Error, Unschedulable or UnschedulableAndUnresolvable.
	failedPlugin string
}

// Code returns code of the Status.
func (s *Status) Code() Code {
	if s == nil {
		return Success
	}
	return s.code
}

// Message returns a concatenated message on reasons of the Status.
func (s *Status) Message() string {
	if s == nil {
		return ""
	}
	return strings.Join(s.reasons, ", ")
}

// SetFailedPlugin sets the given plugin name to s.failedPlugin.
func (s *Status) SetFailedPlugin(plugin string) {
	s.failedPlugin = plugin
}

// WithFailedPlugin sets the given plugin name to s.failedPlugin,
// and returns the given status object.
func (s *Status) WithFailedPlugin(plugin string) *Status {
	s.SetFailedPlugin(plugin)
	return s
}

// FailedPlugin returns the failed plugin name.
func (s *Status) FailedPlugin() string {
	return s.failedPlugin
}

// Reasons returns reasons of the Status.
func (s *Status) Reasons() []string {
	return s.reasons
}

// AppendReason appends given reason to the Status.
func (s *Status) AppendReason(reason string) {
	s.reasons = append(s.reasons, reason)
}

// IsSuccess returns true if and only if "Status" is nil or Code is "Success".
func (s *Status) IsSuccess() bool {
	return s.Code() == Success
}

// IsUnschedulable returns true if "Status" is Unschedulable (Unschedulable or UnschedulableAndUnresolvable).
func (s *Status) IsUnschedulable() bool {
	code := s.Code()
	return code == Unschedulable || code == UnschedulableAndUnresolvable
}

// AsError returns nil if the status is a success; otherwise returns an "error" object
// with a concatenated message on reasons of the Status.
func (s *Status) AsError() error {
	if s.IsSuccess() {
		return nil
	}
	if s.err != nil {
		return s.err
	}
	return errors.New(s.Message())
}

// Equal checks equality of two statuses. This is useful for testing with
// cmp.Equal.
func (s *Status) Equal(x *Status) bool {
	if s == nil || x == nil {
		return s.IsSuccess() && x.IsSuccess()
	}
	if s.code != x.code {
		return false
	}
	if s.code == Error {
		return cmp.Equal(s.err, x.err, cmpopts.EquateErrors())
	}
	return cmp.Equal(s.reasons, x.reasons)
}

// NewStatus makes a Status out of the given arguments and returns its pointer.
func NewStatus(code Code, reasons ...string) *Status {
	s := &Status{
		code:    code,
		reasons: reasons,
	}
	if code == Error {
		s.err = errors.New(s.Message())
	}
	return s
}

// AsStatus wraps an error in a Status.
func AsStatus(err error) *Status {
	return &Status{
		code:    Error,
		reasons: []string{err.Error()},
		err:     err,
	}
}
