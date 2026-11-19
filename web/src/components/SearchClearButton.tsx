export function SearchClearButton({ onClear }: { onClear: () => void }): React.ReactElement {
  return (
    <button
      type="button"
      className="clear"
      aria-label="Clear filter"
      data-testid="clear-filter"
      onClick={onClear}
    >
      <i className="ph ph-x" aria-hidden="true" />
    </button>
  )
}
