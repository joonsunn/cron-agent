export interface JobStatus {
  name: string;
  schedule: string;
  enabled: boolean;
  nextRun: string | null;
  lastStatus: string | null;
  lastExit: number | null;
  consecutiveFailures: number;
}

export interface RunRecord {
  id: string;
  job: string;
  scheduledAt: string;
  startedAt: string;
  finishedAt: string | null;
  exitCode: number | null;
  status: string;
  trigger: string;
}
