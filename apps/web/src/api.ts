import type { JobStatus, RunRecord } from "@cron-agent/shared";

export async function fetchJobs(): Promise<JobStatus[]> {
  const r = await fetch("/api/jobs");
  if (!r.ok) throw new Error("jobs failed");
  return r.json();
}

export async function fetchRuns(job = ""): Promise<RunRecord[]> {
  const q = job ? `?job=${encodeURIComponent(job)}&limit=50` : "?limit=50";
  const r = await fetch(`/api/runs${q}`);
  if (!r.ok) throw new Error("runs failed");
  return r.json();
}

export async function postAction(job: string, action: "trigger" | "pause" | "resume"): Promise<void> {
  const r = await fetch(`/api/jobs/${encodeURIComponent(job)}/${action}`, { method: "POST" });
  if (!r.ok) throw new Error(await r.text());
}
