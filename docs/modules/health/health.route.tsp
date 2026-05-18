import "@typespec/http";
import "./health.model.tsp";

using TypeSpec.Http;
using Api.Models;

namespace Api;

@route("/health")
interface HealthRoutes {
  /** Returns server health, DB ping, Redis ping, memory stats, and uptime. */
  @get
  @summary("Health check")
  health(): HealthData;
}
