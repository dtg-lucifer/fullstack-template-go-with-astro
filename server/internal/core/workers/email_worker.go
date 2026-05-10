// Package workers contains job processor functions consumed by the queue manager.
// Each worker file owns one job type — add a new file here for each new job kind.
//
// Workers are pure functions that receive a typed job struct and return an error.
// They are passed as handler callbacks to queue.Manager.ConsumeXxx methods in
// server.go, keeping the queue wiring and the processing logic separate.
package workers

import (
	"github.com/your-username/go-mux-backend-template/server/internal/core/queue"
	"github.com/your-username/go-mux-backend-template/server/pkg"
)

// WelcomeEmailHandler returns a handler function for WelcomeEmailJob messages.
// The returned function is passed directly to queue.Manager.ConsumeWelcomeEmails.
//
// Replace the body with a real email-send call in production, e.g.:
//
//	return mailer.SendWelcome(job.Email)
func WelcomeEmailHandler(logger *pkg.Logger) func(queue.WelcomeEmailJob) error {
	return func(job queue.WelcomeEmailJob) error {
		// TODO: call your email service here, e.g. mailer.SendWelcome(job.Email)
		logger.Info("[WORKER] Sending welcome email",
			"user_id", job.UserID,
			"email", job.Email,
		)
		return nil
	}
}
