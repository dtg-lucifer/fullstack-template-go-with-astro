package events

// ── Payload types ─────────────────────────────────────────────────────────────

// AuditLogPayload is emitted by any authenticated handler that wants to record
// an action. The listener writes it to the database asynchronously so the
// request handler never waits for the DB write.
type AuditLogPayload struct {
	ActorUserID string // UUID string; empty for unauthenticated actions
	Action      string // e.g. "login", "register", "update_profile"
	Entity      string // e.g. "user", "post"
	Metadata    []byte // optional JSON blob
	IP          string
	UserAgent   string
}

// ── Typed On/Emit pairs ───────────────────────────────────────────────────────

// OnAuditLog registers a listener for the "audit.log" event.
func (b *Bus) OnAuditLog(fn func(AuditLogPayload)) {
	b.on("audit.log", func(v any) { fn(v.(AuditLogPayload)) })
}

// EmitAuditLog publishes an AuditLogPayload. Fire-and-forget from the caller's
// perspective — the listener handles the DB write in its own goroutine.
func (b *Bus) EmitAuditLog(p AuditLogPayload) {
	b.emit("audit.log", p)
}
