import type { SessionStatus } from "../api";

export interface Step {
  label: string;
  state: "done" | "active" | "todo";
}

/**
 * Map backend session status onto the 5-step launch stepper.
 * The API only reports provisioning/ready/stopping/stopped/error, so while
 * provisioning we honestly show step 1 done and steps 2-4 live (pod work
 * is in flight server-side), step 5 pending until the Get() self-heal flips
 * the record to ready.
 */
export function stepsFor(status: SessionStatus | "creating"): Step[] {
  switch (status) {
    case "creating":
      return [
        { label: "Creating your session", state: "active" },
        { label: "Scheduling the workspace pod", state: "todo" },
        { label: "Pulling image & starting containers", state: "todo" },
        { label: "Starting ROS services", state: "todo" },
        { label: "Ready", state: "todo" },
      ];
    case "provisioning":
      return [
        { label: "Creating your session", state: "done" },
        { label: "Scheduling the workspace pod", state: "active" },
        { label: "Pulling image & starting containers", state: "active" },
        { label: "Starting ROS services", state: "active" },
        { label: "Ready", state: "todo" },
      ];
    case "ready":
      return [
        { label: "Creating your session", state: "done" },
        { label: "Scheduling the workspace pod", state: "done" },
        { label: "Pulling image & starting containers", state: "done" },
        { label: "Starting ROS services", state: "done" },
        { label: "Ready", state: "done" },
      ];
    case "stopping":
      return [
        { label: "Creating your session", state: "done" },
        { label: "Scheduling the workspace pod", state: "done" },
        { label: "Pulling image & starting containers", state: "done" },
        { label: "Starting ROS services", state: "done" },
        { label: "Ready", state: "active" },
      ];
    default:
      return [
        { label: "Creating your session", state: "todo" },
        { label: "Scheduling the workspace pod", state: "todo" },
        { label: "Pulling image & starting containers", state: "todo" },
        { label: "Starting ROS services", state: "todo" },
        { label: "Ready", state: "todo" },
      ];
  }
}

export function Stepper({ steps }: { steps: Step[] }) {
  return (
    <ol className="stepper" aria-label="Workspace provisioning progress">
      {steps.map((s) => (
        <li key={s.label} className={`step step--${s.state}`}>
          <span
            className="step__dot"
            aria-hidden="true"
          >
            {s.state === "done" ? "✓" : s.state === "active" ? "◌" : "○"}
          </span>
          <span className="step__label">{s.label}</span>
          <span className="sr-only">
            {s.state === "done"
              ? "done"
              : s.state === "active"
                ? "in progress"
                : "pending"}
          </span>
        </li>
      ))}
    </ol>
  );
}
