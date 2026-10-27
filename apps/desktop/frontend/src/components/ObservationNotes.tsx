export function ObservationNotes({ warnings }: { warnings: string[] }) {
  const notes = [...new Set(warnings)];
  if (!notes.length) return null;
  return (
    <details className="observation-notes">
      <summary>Partial observation · {notes.length} source note{notes.length === 1 ? "" : "s"}</summary>
      <p>Some sources could not be fully verified. Available measurements are shown; missing values stay unknown.</p>
      <ul>{notes.map((note) => <li key={note}>{note}</li>)}</ul>
    </details>
  );
}
