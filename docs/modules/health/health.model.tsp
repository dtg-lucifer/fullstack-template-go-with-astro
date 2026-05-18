import "../common.tsp";

namespace Api.Models;

model DatabaseStatus {
  status: string;
  open_conns: int32;
  idle_conns: int32;
}

model RedisStatus {
  status: string;
}

model MemoryStats {
  alloc_mb: float64;
  total_alloc_mb: float64;
  sys_mb: float64;
  num_gc: int32;
}

model ServicesStatus {
  database: DatabaseStatus;
  redis: RedisStatus;
}

model HealthData {
  success: boolean;
  status: string;
  uptime: string;
  services: ServicesStatus;
  memory: MemoryStats;
}
