import { useCallback, useEffect, useState } from "react";
import { fetchJobs, fetchRuns, postAction } from "./api";
import type { JobStatus, RunRecord } from "@cron-agent/shared";

export default function App() {
  const [jobs, setJobs] = useState<JobStatus[]>([]);
  const [runs, setRuns] = useState<RunRecord[]>([]);
  const [selected, setSelected] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      setJobs(await fetchJobs());
      setRuns(await fetchRuns(selected));
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : "load failed");
    }
  }, [selected]);

  useEffect(() => {
    load();
    const t = setInterval(load, 10000);
    return () => clearInterval(t);
  }, [load]);

  const act = async (job: string, action: "trigger" | "pause" | "resume") => {
    await postAction(job, action);
    await load();
  };

  return (
    <main style={{ fontFamily: "system-ui", maxWidth: 960, margin: "0 auto", padding: 24 }}>
      <h1>cron-agent</h1>
      {error && <p role="alert">{error}</p>}
      <section>
        <h2>Jobs</h2>
        <ul>
          {jobs.map((j) => (
            <li key={j.name}>
              <strong>{j.name}</strong> {j.schedule} next={j.nextRun ?? "-"} last=
              {j.lastStatus ?? "-"}
              {j.consecutiveFailures >= 3 && " NEEDS ATTENTION"}
              <button onClick={() => setSelected(j.name)}>history</button>
              <button onClick={() => act(j.name, "trigger")}>run now</button>
              <button onClick={() => act(j.name, j.enabled ? "pause" : "resume")}>
                {j.enabled ? "pause" : "resume"}
              </button>
            </li>
          ))}
        </ul>
      </section>
      <section>
        <h2>Runs{selected ? ` for ${selected}` : ""}</h2>
        {selected && <button onClick={() => setSelected("")}>clear</button>}
        <ul>
          {runs.map((r) => (
            <li key={r.id}>
              {r.startedAt} {r.job} {r.status} {r.trigger}{" "}
              <a href={`/api/runs/${encodeURIComponent(r.id)}/log`} target="_blank" rel="noreferrer">
                log
              </a>
            </li>
          ))}
        </ul>
      </section>
    </main>
  );
}
