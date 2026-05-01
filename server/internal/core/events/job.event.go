package events

// ── Payload types ─────────────────────────────────────────────────────────────

// JobEnqueuedPayload is emitted after a job is published to the queue.
type JobEnqueuedPayload struct {
	Queue   string
	JobName string
	JobID   string
}

// ── Typed On/Emit pairs ───────────────────────────────────────────────────────

func (b *Bus) OnJobEnqueued(fn func(JobEnqueuedPayload)) {
	b.on("queue.job.enqueued", func(v any) { fn(v.(JobEnqueuedPayload)) })
}

func (b *Bus) EmitJobEnqueued(p JobEnqueuedPayload) {
	b.emit("queue.job.enqueued", p)
}
