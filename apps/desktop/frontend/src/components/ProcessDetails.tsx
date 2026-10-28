import { useState } from "react";
import { X, AlertTriangle, Shield, Power, Trash2, Gauge } from "lucide-react";
import type { ProcessRow } from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/models";
import * as ProcessControlService from "../../bindings/github.com/Caseymccallum/StackBroom/apps/desktop/processcontrolservice";
import { Badge, Button } from "./ui";

interface ProcessDetailsProps {
  process: ProcessRow;
  onClose: () => void;
}

export function ProcessDetails({ process, onClose }: ProcessDetailsProps) {
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<{ success: boolean; outcome: string; details: string } | null>(null);

  const handleAction = async (action: string, priority?: number) => {
    setLoading(true);
    setResult(null);

    try {
      // Step 1: Get fresh process details with creation time
      const details = await ProcessControlService.GetProcessDetails(process.pid, process.startedAt || new Date().toISOString());

      // Step 2: Create preview
      const previewResult = await ProcessControlService.PreviewProcessAction(
        process.pid,
        action,
        priority || 0,
        details?.createTime || new Date().toISOString()
      );

      if (!previewResult || !previewResult.operationId) {
        throw new Error("Failed to create preview");
      }

      // Step 3: Confirm with user
      const confirmed = window.confirm(
        `Confirm ${action.replace('_', ' ')} for process ${process.name} (PID: ${process.pid})?\n\nThis action cannot be undone.`
      );

      if (!confirmed) {
        setResult({ success: false, outcome: "cancelled", details: "Action cancelled by user" });
        return;
      }

      // Step 4: Execute
      const executionResult = await ProcessControlService.ConfirmProcessAction(previewResult.operationId);

      if (executionResult) {
        setResult({
          success: executionResult.success || false,
          outcome: executionResult.outcome || "unknown",
          details: executionResult.details || "No details provided"
        });
      } else {
        throw new Error("Execution returned no result");
      }
    } catch (error) {
      setResult({
        success: false,
        outcome: "error",
        details: error instanceof Error ? error.message : "Unknown error occurred"
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-surface rounded-lg shadow-lg max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
        <div className="p-6">
          <div className="flex items-start justify-between mb-4">
            <div>
              <h2 className="text-xl font-semibold">{process.name}</h2>
              <p className="text-sm text-muted">PID: {process.pid}</p>
            </div>
            <button onClick={onClose} className="p-2 hover:bg-surface-2 rounded">
              <X size={20} />
            </button>
          </div>

          <div className="space-y-4">
            <div>
              <h3 className="text-sm font-semibold mb-2">Process Identity</h3>
              <div className="bg-surface-2 rounded p-3 space-y-1">
                <div className="flex justify-between">
                  <span className="text-muted">Name:</span>
                  <span className="font-mono">{process.name}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted">PID:</span>
                  <span className="font-mono">{process.pid}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-muted">Category:</span>
                  <Badge>{process.category}</Badge>
                </div>
              </div>
            </div>

            <div>
              <h3 className="text-sm font-semibold mb-2">Available Actions</h3>
              <div className="space-y-2">
                <Button onClick={() => handleAction("graceful_close")} disabled={loading} className="w-full justify-start">
                  <Power size={16} className="mr-2" />
                  Close Gracefully
                </Button>

                <Button onClick={() => handleAction("terminate")} disabled={loading} className="w-full justify-start" style={{ backgroundColor: "var(--red)", color: "white" }}>
                  <Trash2 size={16} className="mr-2" />
                  End Process
                </Button>

                <div className="pt-2 border-t border-border">
                  <p className="text-xs text-muted mb-2">Priority Class:</p>
                  <div className="grid grid-cols-3 gap-2">
                    {["Idle", "Below Normal", "Normal", "Above Normal", "High"].map((p) => (
                      <Button key={p} onClick={() => handleAction("set_priority", priorityValue(p))} disabled={loading} variant="ghost">
                        <Gauge size={12} className="mr-1" />
                        {p}
                      </Button>
                    ))}
                  </div>
                </div>
              </div>
            </div>

            {result && (
              <div className={`p-3 rounded ${result.success ? "bg-success/10 text-success" : "bg-error/10 text-error"}`}>
                <div className="flex items-start gap-2">
                  {result.success ? <Shield size={16} /> : <AlertTriangle size={16} />}
                  <div>
                    <p className="font-semibold">{result.outcome}</p>
                    <p className="text-sm">{result.details}</p>
                  </div>
                </div>
              </div>
            )}

            <div className="bg-amber-500/10 border border-amber-500/20 rounded p-3">
              <div className="flex items-start gap-2">
                <AlertTriangle size={16} className="text-amber-500 mt-0.5" />
                <div className="text-sm">
                  <p className="font-semibold text-amber-700 dark:text-amber-400">Warning</p>
                  <p className="text-amber-600 dark:text-amber-300">
                    Forcibly terminating a process may result in data loss.
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

function priorityValue(name: string): number {
  switch (name) {
    case "Idle": return 0x00000040;
    case "Below Normal": return 0x00004000;
    case "Normal": return 0x00000020;
    case "Above Normal": return 0x00008000;
    case "High": return 0x00000080;
    default: return 0x00000020;
  }
}
