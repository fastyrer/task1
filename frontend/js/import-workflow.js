// import-workflow.js - предпросмотр и финальное подтверждение локального импорта.

import { API, postJSON } from "./api.js";
import { saveActiveDraft } from "./draft-store.js";
import { renderFileResult } from "./file-view.js";
import { setButtonLabel } from "./icons.js";
import {
  appState,
  conflictDecision,
  conflictDecisions,
  currentDraft,
  setConflictDecision,
  setDraftState,
  setImportPreview,
  updateDraftRows,
} from "./state.js";

const saveSection = document.getElementById("saveSection");
const saveButton = document.getElementById("saveButton");
const saveStatus = document.getElementById("saveStatus");
const saveError = document.getElementById("saveError");
const saveResult = document.getElementById("saveResult");
const saveStats = document.getElementById("saveStats");
const invalidRowsTable = document.getElementById("invalidRowsTable");
const fixButton = document.getElementById("fixRowsButton");
const fixStatus = document.getElementById("fixStatus");
const fixError = document.getElementById("fixError");
const validateDraftButton = document.getElementById("validateDraftButton");
const draftEditStatus = document.getElementById("draftEditStatus");
const conflictsSection = document.getElementById("conflictsSection");
const conflictsBlock = document.getElementById("conflictsBlock");
const batchResolveRow = document.getElementById("batchResolveRow");
const resolveAllSkip = document.getElementById("resolveAllSkip");
const resolveAllReplace = document.getElementById("resolveAllReplace");
const resolveAllMerge = document.getElementById("resolveAllMerge");
const commitRow = document.getElementById("commitRow");
const commitStatus = document.getElementById("commitStatus");
const commitButton = document.getElementById("commitButton");
const conflictPagination = document.getElementById("conflictPagination");
const conflictPageInfo = document.getElementById("conflictPageInfo");
const previousConflictPage = document.getElementById("previousConflictPage");
const nextConflictPage = document.getElementById("nextConflictPage");

const conflictsPerPage = 25;
let currentConflictPage = 0;

export function initImportWorkflow() {
  saveButton.addEventListener("click", previewImport);
  fixButton.addEventListener("click", validateEditedRows);
  validateDraftButton.addEventListener("click", () => validateCurrentDraft());
  resolveAllSkip.addEventListener("click", () => resolveAllConflicts("skip"));
  resolveAllReplace.addEventListener("click", () => resolveAllConflicts("replace"));
  resolveAllMerge.addEventListener("click", () => resolveAllConflicts("merge"));
  previousConflictPage.addEventListener("click", () => changeConflictPage(currentConflictPage - 1));
  nextConflictPage.addEventListener("click", () => changeConflictPage(currentConflictPage + 1));
  commitButton.addEventListener("click", commitImport);
  document.addEventListener("draft:edited", markDraftAsDirty);

  return { showImportPanel, reset };
}

function showImportPanel(validation = {}) {
  saveSection.classList.remove("is-hidden");
  saveResult.classList.add("is-hidden");
  conflictsSection.classList.add("is-hidden");
  conflictPagination.classList.add("is-hidden");
  commitRow.classList.add("is-hidden");
  saveError.textContent = "";

  const filename = validation.draft?.originalFilename || "Локальный черновик";
  const validCount = validation.stats?.validRowCount;
  const invalidCount = validation.stats?.invalidRowCount || 0;
  saveStatus.textContent = invalidCount > 0
    ? `${filename} · сначала исправьте строк: ${invalidCount}`
    : `${filename} · к проверке готово строк: ${validCount || 0}`;
  saveButton.disabled = invalidCount > 0 || appState.dirty;
  setButtonLabel(saveButton, "Проверить конфликты");
}

function markDraftAsDirty() {
  saveButton.disabled = true;
  saveStatus.textContent = "Проверьте локальные изменения перед сравнением с PostgreSQL.";
  saveResult.classList.add("is-hidden");
  conflictPagination.classList.add("is-hidden");
  commitRow.classList.add("is-hidden");
  draftEditStatus.textContent = "Есть непроверенные локальные изменения";
}

function reset() {
  saveSection.classList.add("is-hidden");
  saveResult.classList.add("is-hidden");
  conflictsSection.classList.add("is-hidden");
  conflictPagination.classList.add("is-hidden");
  commitRow.classList.add("is-hidden");
  saveStats.replaceChildren();
  conflictsBlock.replaceChildren();
  currentConflictPage = 0;
  saveError.textContent = "";
}

async function previewImport() {
  const draft = currentDraft();
  if (!draft) {
    saveError.textContent = "Локальный черновик не найден.";
    return;
  }

  saveError.textContent = "";
  saveButton.disabled = true;
  setButtonLabel(saveButton, "Проверка...");
  saveStatus.textContent = "Сравниваем телефоны с PostgreSQL...";
  try {
    const { response, data } = await postJSON(API.previewImport, { draft });
    if (!response.ok) {
      saveError.textContent = data.error || "Не удалось проверить конфликты.";
      resetPreviewButton();
      return;
    }

    setImportPreview(data);
    renderPreviewStats(data);
    renderConflicts(data.conflicts || []);
    saveResult.classList.remove("is-hidden");
    commitRow.classList.remove("is-hidden");
    saveStatus.textContent = "Предпросмотр готов. PostgreSQL не изменён.";
    setButtonLabel(saveButton, "Проверить повторно");
    saveButton.disabled = false;
    updateCommitAvailability();
  } catch (error) {
    saveError.textContent = "Не удалось подключиться к серверу.";
    resetPreviewButton();
  }
}

function renderPreviewStats(preview) {
  saveStats.replaceChildren();
  appendSaveStat("ok", `Новых: ${preview.newCount || 0}`);
  appendSaveStat("ok", `Совпадают: ${preview.matchedCount || 0}`);
  appendSaveStat("warn", `Без телефона: ${preview.skippedCount || 0}`);
  appendSaveStat(preview.conflictCount > 0 ? "danger" : "ok", `Конфликтов: ${preview.conflictCount || 0}`);
}

function appendSaveStat(status, text) {
  const item = document.createElement("div");
  item.className = `save-stat ${status}`;
  item.textContent = text;
  saveStats.appendChild(item);
}

function resetPreviewButton() {
  saveButton.disabled = false;
  setButtonLabel(saveButton, "Проверить конфликты");
  saveStatus.textContent = "Черновик не был записан в PostgreSQL.";
}

function renderConflicts(conflicts) {
  conflictsBlock.replaceChildren();
  currentConflictPage = 0;
  if (!conflicts.length) {
    conflictsSection.classList.add("is-hidden");
    batchResolveRow.classList.add("is-hidden");
    conflictPagination.classList.add("is-hidden");
    return;
  }

  conflictsSection.classList.remove("is-hidden");
  batchResolveRow.classList.remove("is-hidden");
  renderConflictPage(conflicts);
}

// renderConflictPage создаёт в DOM не больше 25 карточек, сохраняя решения всех страниц в appState.
function renderConflictPage(conflicts) {
  const pageCount = Math.max(1, Math.ceil(conflicts.length / conflictsPerPage));
  currentConflictPage = Math.min(Math.max(currentConflictPage, 0), pageCount - 1);
  const start = currentConflictPage * conflictsPerPage;
  const pageConflicts = conflicts.slice(start, start + conflictsPerPage);
  conflictsBlock.replaceChildren();

  pageConflicts.forEach((conflict, pageIndex) => {
    const index = start + pageIndex;
    const card = document.createElement("div");
    card.className = "conflict-card";
    card.dataset.index = index;

    const phoneRow = document.createElement("div");
    phoneRow.className = "conflict-phone";
    phoneRow.textContent = `Строка ${conflict.row}: ${conflict.phone}`;
    card.appendChild(phoneRow);

    const table = document.createElement("table");
    table.className = "conflict-diff-table";
    table.append(createConflictHeader(), createConflictBody(conflict));
    card.appendChild(table);

    const actions = document.createElement("div");
    actions.className = "conflict-actions";
    actions.append(
      createConflictButton("skip", "Пропустить", conflict, index),
      createConflictButton("replace", "Заменить", conflict, index),
      createConflictButton("merge", "Объединить", conflict, index),
    );
    card.appendChild(actions);
    conflictsBlock.appendChild(card);

    const decision = conflictDecision(conflict.phone);
    if (decision) {
      markConflictResolved(card, decision.action);
    }
  });

  conflictPagination.classList.toggle("is-hidden", pageCount <= 1);
  conflictPageInfo.textContent = `Страница ${currentConflictPage + 1} из ${pageCount} · конфликты ${start + 1}–${start + pageConflicts.length} из ${conflicts.length}`;
  previousConflictPage.disabled = currentConflictPage === 0;
  nextConflictPage.disabled = currentConflictPage >= pageCount - 1;
}

function changeConflictPage(page) {
  const conflicts = appState.importPreview?.conflicts || [];
  const pageCount = Math.ceil(conflicts.length / conflictsPerPage);
  if (page < 0 || page >= pageCount || page === currentConflictPage) {
    return;
  }
  currentConflictPage = page;
  renderConflictPage(conflicts);
  conflictsSection.scrollIntoView({ behavior: "smooth", block: "start" });
}

function createConflictHeader() {
  const thead = document.createElement("thead");
  const row = document.createElement("tr");
  ["Поле", "В PostgreSQL", "В файле"].forEach((text) => {
    const cell = document.createElement("th");
    cell.textContent = text;
    row.appendChild(cell);
  });
  thead.appendChild(row);
  return thead;
}

function createConflictBody(conflict) {
  const tbody = document.createElement("tbody");
  const allKeys = new Set([
    ...Object.keys(conflict.existing || {}),
    ...Object.keys(conflict.incoming || {}),
  ]);
  allKeys.forEach((key) => {
    if (key === "phone") {
      return;
    }
    const existingValue = conflict.existing?.[key] || "";
    const incomingValue = conflict.incoming?.[key] || "";
    if (!existingValue && !incomingValue) {
      return;
    }
    const row = document.createElement("tr");
    if (existingValue !== incomingValue) {
      row.className = "diff";
    }
    [key, existingValue || "(пусто)", incomingValue || "(пусто)"].forEach((value) => {
      const cell = document.createElement("td");
      cell.textContent = value;
      row.appendChild(cell);
    });
    tbody.appendChild(row);
  });
  return tbody;
}

function createConflictButton(action, label, conflict, index) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = `btn-outline ${action}`;
  button.textContent = label;
  button.addEventListener("click", () => resolveConflict(conflict, action, index));
  return button;
}

function resolveConflict(conflict, action, index) {
  setConflictDecision(conflict, action);
  markConflictResolved(conflictsBlock.querySelector(`[data-index="${index}"]`), action);
  updateCommitAvailability();
}

function resolveAllConflicts(action) {
  const conflicts = appState.importPreview?.conflicts || [];
  conflicts.forEach((conflict, index) => {
    setConflictDecision(conflict, action);
    markConflictResolved(conflictsBlock.querySelector(`[data-index="${index}"]`), action);
  });
  updateCommitAvailability();
}

function markConflictResolved(card, action) {
  if (!card) {
    return;
  }
  card.dataset.resolved = "true";
  const status = document.createElement("span");
  status.className = "conflict-resolution";
  status.textContent = getActionLabel(action);
  card.querySelector(".conflict-actions")?.replaceChildren(status);
}

function getActionLabel(action) {
  switch (action) {
    case "skip": return "Будет пропущено";
    case "replace": return "Будет заменено";
    case "merge": return "Будет объединено";
    default: return action;
  }
}

function updateCommitAvailability() {
  const conflictCount = appState.importPreview?.conflictCount || 0;
  const resolvedCount = conflictDecisions().length;
  const unresolved = Math.max(0, conflictCount - resolvedCount);
  commitButton.disabled = unresolved > 0;
  commitStatus.textContent = unresolved > 0
    ? `Осталось выбрать решений: ${unresolved}`
    : "Все решения готовы. Финальная кнопка изменит PostgreSQL.";
  batchResolveRow.classList.toggle("is-hidden", conflictCount === 0 || unresolved === 0);
}

async function commitImport() {
  const draft = currentDraft();
  if (!draft || !appState.importPreview || commitButton.disabled) {
    return;
  }
  if (!window.confirm("Импортировать проверенный файл и применить изменения к PostgreSQL?")) {
    return;
  }

  saveError.textContent = "";
  commitButton.disabled = true;
  setButtonLabel(commitButton, "Импорт...");
  commitStatus.textContent = "Выполняется единая транзакция PostgreSQL...";
  try {
    const { response, data } = await postJSON(API.commitImport, {
      draft,
      decisions: conflictDecisions(),
    });
    if (!response.ok) {
      saveError.textContent = data.error || "Импорт не выполнен.";
      if (response.status === 409) {
        invalidatePreview();
        return;
      }
      setButtonLabel(commitButton, "Импортировать контакты");
      updateCommitAvailability();
      return;
    }

    renderCommitStats(data);
    conflictsSection.classList.add("is-hidden");
    conflictPagination.classList.add("is-hidden");
    commitRow.classList.add("is-hidden");
    saveButton.disabled = true;
    setButtonLabel(saveButton, "Импорт завершён");
    saveStatus.textContent = `Импорт ${data.importId} сохранён атомарно.`;
    document.dispatchEvent(new CustomEvent("import:committed"));
  } catch (error) {
    saveError.textContent = "Не удалось подключиться к серверу.";
    setButtonLabel(commitButton, "Импортировать контакты");
    updateCommitAvailability();
  }
}

// После конкурентного изменения старые версии контактов и решения больше небезопасны.
function invalidatePreview() {
  setImportPreview(null);
  conflictsSection.classList.add("is-hidden");
  conflictPagination.classList.add("is-hidden");
  commitRow.classList.add("is-hidden");
  saveResult.classList.add("is-hidden");
  saveButton.disabled = false;
  setButtonLabel(saveButton, "Проверить конфликты повторно");
  saveStatus.textContent = "Данные PostgreSQL изменились. Нужна новая проверка конфликтов.";
}

function renderCommitStats(result) {
  saveStats.replaceChildren();
  appendSaveStat("ok", `Создано: ${result.created || 0}`);
  appendSaveStat("ok", `Совпало: ${result.matched || 0}`);
  appendSaveStat("warn", `Пропущено: ${result.skipped || 0}`);
  appendSaveStat("ok", `Заменено: ${result.replaced || 0}`);
  appendSaveStat("ok", `Объединено: ${result.merged || 0}`);
}

async function validateEditedRows() {
  fixError.textContent = "";
  if (!currentDraft()) {
    return;
  }
  await validateCurrentDraft([], true);
}

async function validateCurrentDraft(changes = [], fromInvalidRows = false) {
  if (changes.length > 0) {
    updateDraftRows(changes);
  }
  if (!currentDraft()) {
    return;
  }

  const triggerButton = fromInvalidRows ? fixButton : validateDraftButton;
  const statusNode = fromInvalidRows ? fixStatus : draftEditStatus;
  triggerButton.disabled = true;
  setButtonLabel(triggerButton, "Проверка...");
  statusNode.textContent = "Повторно нормализуем локальные данные...";
  try {
    const { response, data } = await postJSON(API.validateImport, { draft: currentDraft() });
    if (!response.ok) {
      const message = data.error || "Не удалось проверить изменения.";
      if (fromInvalidRows) {
        fixError.textContent = message;
      } else {
        saveError.textContent = message;
      }
      return;
    }

    await saveActiveDraft(data, false);
    setDraftState(data);
    renderFileResult(data);
    showImportPanel(data);
    const status = data.stats?.invalidRowCount > 0
      ? `Осталось строк с ошибками: ${data.stats.invalidRowCount}`
      : "Все строки прошли повторную проверку.";
    fixStatus.textContent = status;
    draftEditStatus.textContent = status;
  } catch (error) {
    const message = "Не удалось проверить или сохранить локальные изменения.";
    if (fromInvalidRows) {
      fixError.textContent = message;
    } else {
      saveError.textContent = message;
    }
  } finally {
    triggerButton.disabled = false;
    setButtonLabel(triggerButton, fromInvalidRows ? "Проверить исправления" : "Проверить изменения");
  }
}
