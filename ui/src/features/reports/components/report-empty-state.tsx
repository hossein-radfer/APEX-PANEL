// ReportEmptyState moved to the shared ui/ kit as EmptyState (see that
// file's own doc comment) since it turned out to be feature-agnostic, not
// Reports-specific. Re-exported under its original name here so every
// existing Reports import site keeps working unchanged.
export { EmptyState as ReportEmptyState } from '@/components/ui/empty-state.tsx'
