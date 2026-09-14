import { useCallback, useEffect, useState } from "react";
import { fetchJobs, fetchRuns, postAction } from "./api";
import type { JobStatus, RunRecord } from "@cron-agent/shared";

const POLL_MS = 10000;

function fmtClock(iso: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString();
}

function fmtCountdown(nextRun: string | null, now: number): string {
  if (!nextRun) return "—";
  const t = Date.parse(nextRun);
  if (Number.isNaN(t)) return nextRun;
  const diff = t - now;
  if (diff <= 0) return "due now";
  const s = Math.floor(diff / 1000);
  if (s < 60) return `in ${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `in ${m}m ${s % 60}s`;
  const h = Math.floor(m / 60);
  if (h < 48) return `in ${h}h ${m % 60}m`;
  return `in ${Math.floor(h / 24)}d ${h % 24}h`;
}

function nextRunHot(nextRun: string | null, now: number): boolean {
  if (!nextRun) return false;
  const t = Date.parse(nextRun);
  return !Number.isNaN(t) && t - now > 0 && t - now < 5 * 60 * 1000;
}

function fmtDuration(startedAt: string, finishedAt: string | null): string {
  if (!finishedAt) return "…";
  const ms = Date.parse(finishedAt) - Date.parse(startedAt);
  if (Number.isNaN(ms) || ms < 0) return "—";
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  return `${Math.floor(s / 60)}m ${s % 60}s`;
}

function stampClass(status: string): string {
  if (status === "ok") return "stamp stamp-ok";
  if (status === "running") return "stamp stamp-running";
  if (status === "timeout" || status === "error") return "stamp stamp-bad";
  return "stamp stamp-mute";
}

function jobNeedsAttention(j: JobStatus): boolean {
  return j.enabled && j.consecutiveFailures >= 3;
}

function jobState(j: JobStatus): { label: string; className: string } {
  if (!j.enabled) return { label: "paused", className: "console-state console-state-dim" };
  if (jobNeedsAttention(j)) return { label: "needs attention", className: "console-state console-state-bad" };
  if (j.lastStatus === "running") return { label: "running", className: "console-state" };
  if (j.lastStatus === "timeout" || j.lastStatus === "error")
    return { label: j.lastStatus, className: "console-state console-state-bad" };
  if (j.lastStatus === "ok") return { label: "watching", className: "console-state" };
  return { label: "idle", className: "console-state console-state-dim" };
}

export default function App() {
  const [jobs, setJobs] = useState<JobStatus[]>([]);
  const [runs, setRuns] = useState<RunRecord[]>([]);
  const [selected, setSelected] = useState("");
  const [error, setError] = useState("");
  const [lastSync, setLastSync] = useState(0);
  const [syncCount, setSyncCount] = useState(0);
  const [now, setNow] = useState(() => Date.now());

  const load = useCallback(async () => {
    try {
      const [j, r] = await Promise.all([fetchJobs(), fetchRuns(selected)]);
      setJobs(j);
      setRuns(r);
      setLastSync(Date.now());
      setSyncCount((c) => c + 1);
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "load failed");
    }
  }, [selected]);

  useEffect(() => {
    load();
    const t = setInterval(load, POLL_MS);
    return () => clearInterval(t);
  }, [load]);

  useEffect(() => {
    const t = setInterval(() => setNow(Date.now()), 500);
    return () => clearInterval(t);
  }, []);

  const act = async (job: string, action: "trigger" | "pause" | "resume") => {
    try {
      await postAction(job, action);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "action failed");
    }
  };

  const attention = jobs.filter(jobNeedsAttention).length;
  const paused = jobs.filter((j) => !j.enabled).length;
  const lampClass = attention > 0 ? "lamp-dot lamp-dot-bad" : paused > 0 ? "lamp-dot lamp-dot-warn" : "lamp-dot";
  const railPct = lastSync === 0 ? 0 : Math.min(100, ((now - lastSync) / POLL_MS) * 100);

  return (
    <div className="desk">
      <header className="masthead">
        <div className="masthead-row">
          <h1 className="wordmark">
            <span className={lampClass} aria-hidden="true" />
            cron-agent
            <span className="wordmark-sub">night watch</span>
          </h1>
          <p className="fleetline">
            {jobs.length} {jobs.length === 1 ? "job" : "jobs"}
            {attention > 0 && ` · ${attention} needs attention`}
            {paused > 0 && ` · ${paused} paused`}
            {lastSync > 0 && ` · synced ${new Date(lastSync).toLocaleTimeString()}`}
          </p>
        </div>
        <div className="tickrail" aria-hidden="true">
          <div
            key={syncCount}
            className={syncCount > 0 ? "tickrail-fill tickrail-flash" : "tickrail-fill"}
            style={{ width: `${railPct}%` }}
          />
        </div>
      </header>

      {error && (
        <div className="alert" role="alert">
          <span>Sync failed: {error}. Check the server is running, then try again.</span>
          <button className="btn" onClick={load}>
            Retry now
          </button>
        </div>
      )}

      <section className="desksection" aria-label="Jobs">
        <div className="sectionhead">
          <h2>Consoles</h2>
          <span className="sectionhead-note">tick 10s · queue 3 max</span>
        </div>
        {jobs.length === 0 ? (
          <div className="empty">
            <p>No jobs registered. Add a schedule to start watching.</p>
            <p>
              Copy <code>data/jobs/example.yaml</code> to a new file, point it at a prompt, set{" "}
              <code>enabled: true</code> — it registers on the next tick.
            </p>
          </div>
        ) : (
          <ul className="consoles">
            {jobs.map((j) => {
              const state = jobState(j);
              const cardClass = jobNeedsAttention(j)
                ? "console console-attention"
                : j.enabled
                  ? "console"
                  : "console console-paused";
              return (
                <li key={j.name} className={cardClass}>
                  <div className="console-top">
                    <h3 className="console-name">{j.name}</h3>
                    <span className={state.className}>{state.label}</span>
                  </div>
                  <div className="cronstrip" title="cron schedule">
                    {j.schedule}
                  </div>
                  <dl className="console-meta">
                    <div>
                      <dt>Next run</dt>
                      <dd className={nextRunHot(j.nextRun, now) ? "nextrun-hot" : undefined}>
                        {fmtCountdown(j.nextRun, now)}
                      </dd>
                    </div>
                    <div>
                      <dt>Last</dt>
                      <dd>{j.lastStatus ?? "—"}</dd>
                    </div>
                    <div>
                      <dt>Fails</dt>
                      <dd>{j.consecutiveFailures}</dd>
                    </div>
                  </dl>
                  {jobNeedsAttention(j) && (
                    <div>
                      <span className="attention-flag">
                        {j.consecutiveFailures} consecutive failures — check the ledger
                      </span>
                    </div>
                  )}
                  <div className="console-actions">
                    <button className="btn btn-primary" onClick={() => act(j.name, "trigger")}>
                      Run now
                    </button>
                    <button className="btn" onClick={() => setSelected(j.name)}>
                      History
                    </button>
                    <button className="btn" onClick={() => act(j.name, j.enabled ? "pause" : "resume")}>
                      {j.enabled ? "Pause" : "Resume"}
                    </button>
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </section>

      <section className="desksection" aria-label="Run history">
        <div className="sectionhead">
          <h2>{selected ? `Ledger · ${selected}` : "Ledger"}</h2>
          {selected ? (
            <div className="filterbar">
              <span className="mono">{runs.length} runs</span>
              <button className="btn" onClick={() => setSelected("")}>
                Show all
              </button>
            </div>
          ) : (
            <span className="sectionhead-note">newest first · last 50</span>
          )}
        </div>
        {runs.length === 0 ? (
          <div className="empty">
            <p>No runs recorded{selected ? ` for ${selected}` : ""} yet.</p>
            <p>Runs land here after the next due tick — or trigger one now from its console.</p>
          </div>
        ) : (
          <div className="ledger-wrap">
            <table className="ledger">
              <thead>
                <tr>
                  <th scope="col">Started</th>
                  <th scope="col">Job</th>
                  <th scope="col">Status</th>
                  <th scope="col">Took</th>
                  <th scope="col">Trigger</th>
                  <th scope="col">Log</th>
                </tr>
              </thead>
              <tbody>
                {runs.map((r) => (
                  <tr key={r.id}>
                    <td className="mono">{fmtClock(r.startedAt)}</td>
                    <td className="mono">{r.job}</td>
                    <td>
                      <span className={stampClass(r.status)}>{r.status}</span>
                      {r.exitCode !== null && r.status !== "running" && (
                        <span className="mono"> exit {r.exitCode}</span>
                      )}
                    </td>
                    <td className="mono">{fmtDuration(r.startedAt, r.finishedAt)}</td>
                    <td className="mono">{r.trigger}</td>
                    <td>
                      <a
                        className="loglink"
                        href={`/api/runs/${encodeURIComponent(r.id)}/log`}
                        target="_blank"
                        rel="noreferrer"
                      >
                        open log
                      </a>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
