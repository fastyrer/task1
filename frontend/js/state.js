// state.js - состояние локального черновика текущей вкладки.

export const appState = {
  validation: null,
  currentHeaders: [],
  importPreview: null,
  decisions: {},
  dirty: false,
};

// setDraftState заменяет проверенный черновик и сбрасывает устаревший preview конфликтов.
export function setDraftState(validation, dirty = false) {
  appState.validation = validation || null;
  appState.currentHeaders = validation?.draft?.headers || [];
  appState.importPreview = null;
  appState.decisions = {};
  appState.dirty = Boolean(validation && dirty);
}

export function clearDraftState() {
  setDraftState(null);
}

export function currentDraft() {
  return appState.validation?.draft || null;
}

// updateDraftRows переносит изменения из редактируемой таблицы в локальный черновик.
export function updateDraftRows(changes) {
  const draft = currentDraft();
  if (!draft) {
    return;
  }

  const valuesByRow = new Map(changes.map((change) => [change.rowNumber, change.values]));
  draft.rows = draft.rows.map((row) => (
    valuesByRow.has(row.rowNumber)
      ? { ...row, values: valuesByRow.get(row.rowNumber) }
      : row
  ));
  appState.importPreview = null;
  appState.decisions = {};
  appState.dirty = true;
}

export function updateDraftCell(rowNumber, header, value) {
  const row = currentDraft()?.rows.find((item) => item.rowNumber === rowNumber);
  if (!row || !header) {
    return;
  }
  row.values[header] = value;
  appState.importPreview = null;
  appState.decisions = {};
  appState.dirty = true;
}

export function setImportPreview(preview) {
  appState.importPreview = preview || null;
  appState.decisions = {};
}

export function setConflictDecision(conflict, action) {
  appState.decisions[conflict.phone] = {
    phone: conflict.phone,
    action,
    version: conflict.version,
  };
}

export function conflictDecisions() {
  return Object.values(appState.decisions);
}

export function conflictDecision(phone) {
  return appState.decisions[phone] || null;
}
