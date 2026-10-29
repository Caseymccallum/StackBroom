const dayReason = /^newer than (\d+) days$/;

export function cleanupStatus(status: string): string {
  switch (status) {
    case "running":
      return "Searching";
    case "completed":
      return "Complete";
    case "cancelled":
      return "Stopped";
    case "timeout":
      return "Took too long";
    case "error":
      return "Couldn't search";
    default:
      return "Searching";
  }
}

export function cleanupReason(reason: string): string {
  const age = reason.match(dayReason);
  if (age) return `Changed in the last ${age[1]} days`;
  switch (reason) {
    case "extension excluded by rule":
      return "Not a crash dump";
    case "hard-linked file":
      return "Linked to another file";
    case "reparse point":
      return "Shortcut or linked location";
    case "cloud placeholder":
      return "Stored in the cloud";
    case "target is no longer old enough":
      return "Changed too recently";
    case "target extension no longer matches the rule":
      return "No longer a crash dump";
    case "target has multiple hard links":
      return "Linked to another file";
    case "target is a reparse point":
      return "Shortcut or linked location";
    case "target is a cloud-only file":
      return "Stored in the cloud";
    case "target is outside the approved root or has a reparse ancestor":
      return "Outside the approved folder";
    case "target metadata changed":
      return "Changed since the scan";
    default:
      return "Not eligible";
  }
}

export function cleanupError(message: string | null | undefined): string {
  if (!message) return "Something went wrong. Try again.";
  if (/preview expired|review expired/i.test(message)) {
    return "This review expired. Find files again.";
  }
  if (/rule changed|scope|trusted root/i.test(message)) {
    return "The cleanup list changed. Find files again.";
  }
  if (/no selected|not safe|changed before|candidate/i.test(message)) {
    return "None of the selected files can be reviewed. Find files again.";
  }
  if (/busy|shutting down/i.test(message)) {
    return "StackBroom is busy. Try again in a moment.";
  }
  return "Something went wrong. Try again.";
}
