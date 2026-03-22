const API_BASE = 'http://localhost:8080';

export const api = {
  async get<T>(path: string): Promise<T> {
    const res = await fetch(API_BASE + path);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
  },

  async post(path: string, body: unknown): Promise<unknown> {
    const res = await fetch(API_BASE + path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
  },

  async delete(path: string): Promise<unknown> {
    const res = await fetch(API_BASE + path, {
      method: 'DELETE',
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
  },
};

export interface Station {
  station_id: number;
  variant_key: string;
  mount: string;
  enabled: boolean;
  last_seen: string;
  frames_out: number;
  bytes_out: number;
  source_ip: string;
}

export interface StationsResponse {
  total: number;
  stations: Station[];
}

export interface Stats {
  sources: number;
  stations: number;
  forwarded: number;
  unknown: number;
  ambiguous: number;
  drops: number;
  queue_depth: number;
  workers_active: number;
  workers_desired: number;
  uptime: string;
  mem_alloc_mb: number;
  goroutines: number;
}

export interface Health {
  status: string;
  uptime: string;
  goroutine: number;
  mem_mb: number;
  queue_fill: number;
}

export interface WorkerInfo {
  active: number;
  desired: number;
  min: number;
  max: number;
  auto: boolean;
}

export interface Config {
  capture: Record<string, unknown>;
  caster: Record<string, unknown>;
  web: Record<string, unknown>;
  worker: {
    min: number;
    max: number;
    auto_scale: boolean;
  };
  runtime: Record<string, unknown>;
  auto_scale: boolean;
}
