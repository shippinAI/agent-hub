// Action feed and audit retention types (mirror server/internal/actionfeed and auditkeep; 39, 40).
export type ActionPolicy = {
  decision: 'allow' | 'deny'; grant_id?: string; sandbox_level?: string; approver?: string; code?: string; reason?: string;
};

export type ActionEvent = {
  id: number; at: string; category: string; kind: string; call_id?: string; actor: string; machine?: string; task_id?: number;
  op: string; tool?: string; argv?: string[]; cwd?: string; paths?: string[]; args?: unknown; diff?: string;
  status: 'running' | 'ok' | 'error' | 'denied'; exit_code?: number; duration_ms?: number; stdout?: string; stderr?: string;
  out_sha256?: string; truncated?: boolean; error?: string; policy?: ActionPolicy; redactions?: number;
};

// One rendered row: a call's started + finished events folded together (or a single tool/denied event).
export type ActionItem = { key: string; first: ActionEvent; last: ActionEvent; events: ActionEvent[] };

export type AuditCategory = {
  category: string; enabled: boolean; retain_days: number; max_rows: number; max_bytes: number; archive: boolean;
  capture_output: boolean; max_output_bytes: number; description: string; version: number; updated_at: string; updated_by: string;
};

export type AuditPartition = { name: string; category: string; day: string; bytes: number; est_rows: number };

export type AuditArchiveSummary = { category: string; segments: number; rows: number; bytes: number; last: string | null };

export type RetentionView = {
  categories: AuditCategory[]; partitions: AuditPartition[]; archives: AuditArchiveSummary[] | null;
  settings: {
    audit_archive_dir: string; audit_total_max_bytes: number; audit_partition_ahead_days: number;
    audit_prune_interval_seconds: number; audit_on_archive_failure: string; signing: boolean;
  };
};

export type PruneReport = { at: string; archived: unknown[]; dropped: string[]; deleted_rows: number; bytes_after: number; warnings: string[] };
