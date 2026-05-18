import "@typespec/http";
import "./auth.model.tsp";

using TypeSpec.Http;
using Api.Models;

namespace Api;

/** JWT Bearer token — required on protected endpoints. */
@doc("Bearer token (JWT)")
model BearerAuth is Http.BearerAuth;

@route("/auth")
@useAuth(BearerAuth)
interface AuthProtected {
  /** Return the currently authenticated user. */
  @get
  @route("/me")
  @summary("Get current user")
  me(): ApiSuccess<MeData> | ApiError;
}

@route("/auth")
interface AuthPublic {
  /** Create a new user account. */
  @post
  @route("/register")
  @summary("Register")
  register(@body body: RegisterRequest): ApiSuccess<RegisterData> | ApiError;

  /** Authenticate with email + password. Returns access and refresh tokens. */
  @post
  @route("/login")
  @summary("Login")
  login(@body body: LoginRequest): ApiSuccess<LoginData> | ApiError;

  /** Exchange a refresh token for a new access token. */
  @post
  @route("/refresh")
  @summary("Refresh access token")
  refresh(@body body: RefreshRequest): ApiSuccess<RefreshData> | ApiError;
}
