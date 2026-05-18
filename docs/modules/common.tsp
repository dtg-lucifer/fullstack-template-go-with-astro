import "@typespec/http";
import "@typespec/openapi3";

using TypeSpec.Http;

namespace Api.Models;

/** Standard success envelope returned by every endpoint. */
model ApiSuccess<T> {
  success: true;
  message: string;
  data: T;
  status_code: int32;
  request_id: string;
}

/** Standard error envelope returned on failure. */
model ApiError {
  success: false;
  message: string;
  errors?: unknown;
  status_code: int32;
  request_id: string;
}
