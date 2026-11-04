// Types matching cluster-monitor-ms/internal/monitor/model.go exactly

export interface Container {
  name: string;
  service: string;
  image: string;
  state: string;       // running | exited | restarting ...
  health: string;      // healthy | unhealthy | starting | none
  cpu_percent: number;
  mem_bytes: number;
  mem_limit_bytes: number;
  mem_percent: number;
  light: "green" | "yellow" | "red";
}

export interface QueueInfo {
  name: string;
  messages: number;
  consumers: number;
}

export interface RabbitInfo {
  status: string;
  messages_ready: number;
  messages_unacked: number;
  publish_rate: number;
  deliver_rate: number;
  queues: QueueInfo[];
  latency_ms: number;
}

export interface PostgresInfo {
  status: string;
  connections: number;
  max_connections: number;
  usage_percent: number;
  latency_ms: number;
}

export interface RedisInfo {
  status: string;
  connected_clients: number;
  latency_ms: number;
}

export interface Alarm {
  source: string;
  severity: "warning" | "critical";
  message: string;
}

export interface Snapshot {
  generated_at: string;        // ISO-8601
  interval_seconds: number;
  containers: Container[];
  rabbitmq: RabbitInfo;
  postgres: PostgresInfo;
  redis: RedisInfo;
  sources: Record<string, string>; // docker|rabbitmq|postgres|redis -> ok|degraded
  alarms: Alarm[];
}

