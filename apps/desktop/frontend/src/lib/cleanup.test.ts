import { describe, expect, it } from "vitest";
import { cleanupError, cleanupReason, cleanupStatus } from "./cleanup";

describe("cleanup copy", () => {
  it("turns exclusions into plain reasons", () => {
    expect(cleanupReason("newer than 14 days")).toBe("Changed in the last 14 days");
    expect(cleanupReason("hard-linked file")).toBe("Linked to another file");
    expect(cleanupReason("extension excluded by rule")).toBe("Not a crash dump");
    expect(cleanupReason("something internal")).toBe("Not eligible");
  });

  it("uses plain search states", () => {
    expect(cleanupStatus("running")).toBe("Searching");
    expect(cleanupStatus("completed")).toBe("Complete");
    expect(cleanupStatus("timeout")).toBe("Took too long");
    expect(cleanupStatus("error")).toBe("Couldn't search");
  });

  it("does not leak developer-oriented errors", () => {
    expect(cleanupError("cleanup execution context was refused")).toBe("Something went wrong. Try again.");
    expect(cleanupError("cleanup preview expired")).toBe("This review expired. Find files again.");
    expect(cleanupError("cleanup rule changed")).toBe("The cleanup list changed. Find files again.");
    expect(cleanupError("cleanup service is busy or shutting down")).toBe("StackBroom is busy. Try again in a moment.");
    expect(cleanupError(null)).toBe("Something went wrong. Try again.");
  });
});
