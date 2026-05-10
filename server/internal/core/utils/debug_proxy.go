// Package utils provides shared utilities used across the application.
package utils

import (
	"fmt"
	"reflect"
	"time"

	"github.com/your-username/go-mux-backend-template/server/pkg"
)

// Dispatcher is a reflective method dispatcher that logs every call.
// It is the Go equivalent of the TypeScript createDebugProxy utility.
//
// Unlike TypeScript's Proxy (which intercepts any object transparently),
// Go's static type system requires an explicit interface. The pattern used
// here is:
//
//  1. Define a service interface (e.g. AuthServiceIface).
//  2. Implement the interface with a concrete struct (e.g. *AuthService).
//  3. Wrap the concrete struct in a typed logging proxy that also implements
//     the interface (e.g. *AuthServiceDebugProxy).
//  4. Expose a WithDebug constructor on the service that returns the proxy.
//
// Dispatcher provides the shared logging + timing logic so each per-service
// proxy only needs to forward calls — it never duplicates the log boilerplate.
//
// Example usage inside a service proxy:
//
//	func (p *AuthServiceDebugProxy) Register(ctx context.Context, input RegisterInput, r *http.Request) utils.ApiResponse {
//	    results := p.d.Call("Register", ctx, input, r)
//	    return results[0].Interface().(utils.ApiResponse)
//	}
type Dispatcher struct {
	target  any
	prefix  string
	logger  *pkg.Logger
	methods map[string]reflect.Value // pre-built bound method values for speed
}

// NewDispatcher creates a Dispatcher that wraps target and logs every Call
// under the given prefix label (e.g. "AuthService", "UserRepository").
func NewDispatcher(target any, prefix string, logger *pkg.Logger) *Dispatcher {
	targetVal := reflect.ValueOf(target)
	targetType := targetVal.Type()

	methods := make(map[string]reflect.Value, targetType.NumMethod())
	for i := range targetType.NumMethod() {
		name := targetType.Method(i).Name
		methods[name] = targetVal.Method(i)
	}

	return &Dispatcher{
		target:  target,
		prefix:  prefix,
		logger:  logger,
		methods: methods,
	}
}

// Call invokes the named method on the wrapped target with the provided
// arguments, logging start/end/error around the call.
//
// args must match the method's parameter types exactly (same as a direct call).
// Returns the method's return values as []reflect.Value so the caller can
// extract typed results with .Interface().(ConcreteType).
//
// Panics if the method name does not exist on the target — this is intentional
// so misconfigured proxies fail loudly at startup rather than silently at runtime.
func (d *Dispatcher) Call(name string, args ...any) []reflect.Value {
	start := time.Now()
	d.logger.Debug(fmt.Sprintf("[%s.%s] --> START", d.prefix, name),
		"args_count", len(args),
	)

	m, ok := d.methods[name]
	if !ok {
		panic(fmt.Sprintf("[DEBUG_PROXY] method %q not found on %s — check the method name", name, d.prefix))
	}

	in := make([]reflect.Value, len(args))
	for i, a := range args {
		in[i] = reflect.ValueOf(a)
	}

	results := m.Call(in)
	duration := time.Since(start)

	// Inspect the last return value for a non-nil error.
	errType := reflect.TypeFor[error]()
	if len(results) > 0 {
		last := results[len(results)-1]
		if last.Type().Implements(errType) && !last.IsNil() {
			d.logger.Debug(fmt.Sprintf("[%s.%s] <-- ERROR", d.prefix, name),
				"duration", duration.String(),
				"error", last.Interface().(error).Error(),
			)
			return results
		}
	}

	d.logger.Debug(fmt.Sprintf("[%s.%s] <-- END", d.prefix, name),
		"duration", duration.String(),
	)
	return results
}
