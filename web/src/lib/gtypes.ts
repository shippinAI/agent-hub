// Change-policy types (docs/devagotchi/14): policy documents, decisions, change tickets, evidence and reviews.
export type Check = { check: string; field?: string; pattern?: string; not?: string; min_len?: number; file_exists?: boolean;
  max_age_s?: number; contains_id?: boolean; min?: number; max?: number; role?: string; msg?: string };
export type Rule = { id: string; name: string; hook: string; enabled: boolean; priority: number; effect: string;
  when: { events?: string[]; tools?: string[]; classes?: string[]; regions?: string[]; command_re?: string; via_runner?: boolean };
  require?: Check[]; message: string; params?: Record<string, number>; collect?: string[] };
export type PolicyDoc = { classes: Record<string, string[]>; regions: { code: string; name: string; hosts: string[] }[]; rules: Rule[] };
export type DocRow = { tenant_id: string; version: number; sha256: string; author: string; reason: string; created_at: string; doc?: PolicyDoc };
export type Failure = { rule: string; hook: string; message: string; why: string[] };
export type Decision = { allow: boolean; policy_version: number; classes: string[] | null; regions: string[] | null; matched: string[] | null;
  failures?: Failure[]; ticket_id?: string };
export type Answer = { decision_id: string; tenant_id: string; policy_sha256: string; allow: boolean; reason: string; decision: Decision };
export type Ticket = { id: string; tenant_id: string; author: string; title: string; machine: string | null; command: string;
  fields: Record<string, string> | null; lists: Record<string, string[]> | null; sha256: string; status: string; version: number; updated_at: string };
export type Evidence = { kind: string; path: string; sha256: string; excerpt: string; captured_at: string; added_by: string };
export type Review = { reviewer: string; verdict: string; summary: string; findings: string[]; ticket_sha256: string; current: boolean; created_at: string };
export type TicketView = { ticket: Ticket; evidence: Evidence[]; reviews: Review[]; preview?: Answer; preview_error?: string };
export type DecisionEvent = { seq: number; at: string; actor: string; data: { decision_id: string; allow: boolean; tool: string; fp: string;
  machine: string; policy_version: number; matched: string[] | null; failures: { rule: string; why: string[] }[]; ticket: string; tenant: string } };

export const statusTone: Record<string, 'neutral' | 'blue' | 'green' | 'amber' | 'red' | 'violet'> = {
  draft: 'neutral', pending_review: 'violet', passed: 'green', blocked: 'red', findings: 'amber', ran: 'blue',
};
export const verdictTone: Record<string, 'green' | 'red' | 'amber'> = { PASS: 'green', BLOCK: 'red', FINDINGS: 'amber' };
