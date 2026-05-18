import "../common.tsp";

namespace Api.Models;

/** Public user fields returned in API responses — password is never included. */
model SafeUser {
  id: string;
  first_name: string;
  last_name: string;
  email: string;
  verified: boolean;
  created_at: string;
}

// ── Request bodies ────────────────────────────────────────────────────────────

model RegisterRequest {
  first_name: string;
  last_name: string;
  email: string;

  @minLength(8)
  password: string;
}

model LoginRequest {
  email: string;
  password: string;
}

model RefreshRequest {
  refresh_token: string;
}

// ── Response data shapes ──────────────────────────────────────────────────────

model RegisterData {
  user: SafeUser;
}

model LoginData {
  user: SafeUser;
  access_token: string;
  refresh_token: string;
}

model MeData {
  user: SafeUser;
}

model RefreshData {
  access_token: string;
}
