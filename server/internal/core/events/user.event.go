package events

// ── Payload types ─────────────────────────────────────────────────────────────

// UserRegisteredPayload is emitted after a new user account is created.
type UserRegisteredPayload struct {
	UserID string
	Email  string
}

// ── Typed On/Emit pairs ───────────────────────────────────────────────────────

func (b *Bus) OnUserRegistered(fn func(UserRegisteredPayload)) {
	b.on("auth.user.registered", func(v any) { fn(v.(UserRegisteredPayload)) })
}

func (b *Bus) EmitUserRegistered(p UserRegisteredPayload) {
	b.emit("auth.user.registered", p)
}
