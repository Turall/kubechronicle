export type Operation = 'CREATE' | 'UPDATE' | 'DELETE';

export interface Actor {
  username: string;
  groups: string[];
  service_account?: string; // Using camelCase mapping from snake_case API potentially? 
  // API doc says "service_account" in response example but snake_case in description?
  // Response example: "service_account": ""
  source_ip?: string; // Response example: "source_ip"
}

export interface Source {
  tool: string;
}

export interface JsonPatch {
  op: 'add' | 'remove' | 'replace' | 'move' | 'copy' | 'test';
  path: string;
  value?: unknown;
  from?: string; // For move/copy
}

export interface ChangeEvent {
  id: string;
  timestamp: string;
  operation: Operation;
  resource_kind: string;
  namespace: string;
  name: string;
  actor: Actor;
  source: Source;
  diff: JsonPatch[];
  allowed: boolean;
  block_pattern?: string;
  object_snapshot?: unknown; // For DELETE
}

export interface PinatedResponse<T> {
  events: T[]; // API calls it "events" for changes list
  total: number;
  limit: number;
  offset: number;
}

export interface ChangeFilterParams {
  resource_kind?: string;
  namespace?: string;
  name?: string;
  user?: string;
  operation?: Operation;
  start_time?: string;
  end_time?: string;
  allowed?: boolean;
  limit?: number;
  offset?: number;
  sort?: 'asc' | 'desc';
}
