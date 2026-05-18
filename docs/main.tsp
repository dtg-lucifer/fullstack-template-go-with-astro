import "@typespec/http";
import "@typespec/openapi";
import "@typespec/openapi3";

import "./modules/health/health.route.tsp";
import "./modules/auth/auth.route.tsp";

using TypeSpec.Http;
using TypeSpec.OpenAPI;

@service(#{ title: "Go Mux Backend API" })
@info(#{
  version: "1.0.0",
  contact: #{
    name: "API Support",
    email: "support@example.com",
  },
  license: #{
    name: "MIT",
    url: "https://opensource.org/licenses/MIT",
  },
})
@server("http://localhost:8080/api/v1", "Local development server")
namespace Api;
