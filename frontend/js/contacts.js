// contacts.js - сохранение контактов, разрешение конфликтов и исправление строк.

import { API, postJSON } from "./api.js";
import {
  renderEmptyInvalidRows,
  renderInvalidRows,
  renderStats,
  renderWarnings,
} from "./file-view.js";
import { appState } from "./state.js";
import { showError } from "./ui.js";

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

const conflictsSection = document.getElementById("conflictsSection");
const conflictsBlock = document.getElementById("conflictsBlock");
const batchResolveRow = document.getElementById("batchResolveRow");
const resolveAllSkip = document.getElementById("resolveAllSkip");
const resolveAllReplace = document.getElementById("resolveAllReplace");
const resolveAllMerge = document.getElementById("resolveAllMerge");

export function initContacts() {
  saveButton.addEventListener("click", saveContacts);
  fixButton.addEventListener("click", fixInvalidRows);
  resolveAllSkip.addEventListener("click", () => resolveAllConflicts("skip"));
  resolveAllReplace.addEventListener("click", () => resolveAllConflicts("replace"));
  resolveAllMerge.addEventListener("click", () => resolveAllConflicts("merge"));

  return { showSavePanel };
}

function showSavePanel() {
  saveSection.classList.remove("is-hidden");
  saveResult.classList.add("is-hidden");
  saveError.textContent = "";
  saveStatus.textContent = "Нажмите, чтобы сохранить валидные данные в базу";
  saveButton.disabled = false;
  saveButton.textContent = "💾 Сохранить данные в БД";
}

async function saveContacts() {
  saveError.textContent = "";
  saveButton.disabled = true;
  saveButton.textContent = "Сохранение...";
  saveStatus.textContent = "Идет сохранение...";

  try {
    const { response, data } = await postJSON(API.save, {
      fileId: appState.currentFileId,
    });
    if (!response.ok) {
      saveError.textContent = data.error || "Ошибка при сохранении.";
      resetSaveButton();
      return;
    }

    saveResult.classList.remove("is-hidden");
    saveStats.replaceChildren();
    appendSaveStat("ok", `✓ Сохранено: ${data.saved || 0}`);
    appendSaveStat(
      data.skipped > 0 ? "warn" : "ok",
      data.skipped > 0 ? `⚠ Пропущено: ${data.skipped}` : "✓ Пропущено: 0",
    );

    const conflicts = data.conflicts || [];
    if (conflicts.length > 0) {
      appendSaveStat("danger", `! Конфликтов: ${conflicts.length}`);
    }

    saveButton.textContent = "💾 Сохранено";
    saveButton.disabled = true;
    saveStatus.textContent = "Данные сохранены";
    renderConflicts(conflicts);
  } catch (error) {
    saveError.textContent = "Не удалось подключиться к серверу.";
    resetSaveButton();
  }
}

function appendSaveStat(status, text) {
  const item = document.createElement("div");
  item.className = `save-stat ${status}`;
  item.textContent = text;
  saveStats.appendChild(item);
}

function resetSaveButton() {
  saveButton.disabled = false;
  saveButton.textContent = "💾 Сохранить данные в БД";
  saveStatus.textContent = "";
}

function renderConflicts(conflicts) {
  if (!conflicts.length) {
    conflictsSection.classList.add("is-hidden");
    return;
  }

  conflictsSection.classList.remove("is-hidden");
  batchResolveRow.classList.remove("is-hidden");
  conflictsBlock.replaceChildren();

  conflicts.forEach((conflict, index) => {
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
      createConflictButton("skip", "Пропустить", conflict.phone, index),
      createConflictButton("replace", "Заменить", conflict.phone, index),
      createConflictButton("merge", "Объединить", conflict.phone, index),
    );
    card.appendChild(actions);
    conflictsBlock.appendChild(card);
  });
}

function createConflictHeader() {
  const thead = document.createElement("thead");
  const row = document.createElement("tr");
  ["Поле", "Существующее", "Новое"].forEach((text) => {
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
    const existingValue = (conflict.existing || {})[key] || "";
    const incomingValue = (conflict.incoming || {})[key] || "";
    if (existingValue === "" && incomingValue === "") {
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

function createConflictButton(action, label, phone, index) {
  const button = document.createElement("button");
  button.className = `btn-outline ${action}`;
  button.textContent = label;
  button.addEventListener("click", () => resolveConflict(phone, action, index));
  return button;
}

async function resolveConflict(phone, action, index) {
  try {
    const { response, data } = await postJSON(API.resolve, {
      fileId: appState.currentFileId,
      phone,
      action,
    });
    if (!response.ok) {
      showError(data.error || "Ошибка при разрешении конфликта.");
      return;
    }

    const card = conflictsBlock.querySelector(`[data-index="${index}"]`);
    markConflictResolved(card, action);
    const remaining = conflictsBlock.querySelectorAll('.conflict-card:not([data-resolved="true"])');
    if (remaining.length === 0) {
      batchResolveRow.classList.add("is-hidden");
    }
  } catch (error) {
    showError("Не удалось подключиться к серверу.");
  }
}

async function resolveAllConflicts(action) {
  try {
    const { response, data } = await postJSON(API.resolveAll, {
      fileId: appState.currentFileId,
      action,
    });
    if (!response.ok) {
      showError(data.error || "Ошибка при массовом разрешении.");
      return;
    }

    conflictsBlock.querySelectorAll(".conflict-card").forEach((card) => {
      markConflictResolved(card, action);
    });
    batchResolveRow.classList.add("is-hidden");
  } catch (error) {
    showError("Не удалось подключиться к серверу.");
  }
}

function markConflictResolved(card, action) {
  if (!card) {
    return;
  }

  card.dataset.resolved = "true";
  const status = document.createElement("span");
  status.className = "conflict-resolution";
  status.textContent = `✓ ${getActionLabel(action)}`;
  card.querySelector(".conflict-actions")?.replaceChildren(status);
}

function getActionLabel(action) {
  switch (action) {
    case "skip":
      return "Пропущено";
    case "replace":
      return "Заменено";
    case "merge":
      return "Объединено";
    default:
      return action;
  }
}

async function fixInvalidRows() {
  fixError.textContent = "";
  const tbody = invalidRowsTable.querySelector("tbody");
  if (!tbody) {
    return;
  }

  const tableRows = [...tbody.querySelectorAll("tr")];
  const rows = tableRows.flatMap((row) => {
    const rowNumber = Number.parseInt(row.dataset.rowNumber, 10);
    return rowNumber ? [{ rowNumber, values: readEditableValues(row) }] : [];
  });
  if (!rows.length) {
    fixError.textContent = "Нет строк для исправления.";
    return;
  }

  fixButton.disabled = true;
  fixButton.textContent = "Сохранение...";
  fixStatus.textContent = "Идет проверка...";

  try {
    const { response, data } = await postJSON(API.fix, {
      fileId: appState.currentFileId,
      rows,
    });
    if (!response.ok) {
      fixError.textContent = data.error || "Ошибка при сохранении.";
      resetFixButton();
      return;
    }

    if (data.stats) {
      renderStats(data.stats);
    }
    if (Array.isArray(data.warnings)) {
      renderWarnings(data.warnings);
    }
    renderFailedRowsMessage(data.failed || []);

    const fixed = data.fixed || 0;
    const failed = data.failed || [];
    if (fixed > 0) {
      fixStatus.textContent = `✓ Исправлено: ${fixed}. ${failed.length > 0 ? `Ошибок: ${failed.length}` : ""}`;
      fixStatus.style.color = "#0f766e";

      const remainingRows = tableRows.flatMap((row) => {
        const rowNumber = Number.parseInt(row.dataset.rowNumber, 10);
        const failure = failed.find((item) => item.rowNumber === rowNumber);
        if (!failure) {
          return [];
        }
        return [{
          row: rowNumber,
          values: readEditableValues(row),
          errors: failure.errors,
        }];
      });

      if (remainingRows.length > 0) {
        renderInvalidRows(remainingRows);
      } else {
        renderEmptyInvalidRows("Все строки исправлены ✓");
      }
    }

    resetFixButton(false);
  } catch (error) {
    fixError.textContent = "Не удалось подключиться к серверу.";
    resetFixButton();
  }
}

function readEditableValues(row) {
  const values = {};
  row.querySelectorAll("td[contenteditable]").forEach((cell) => {
    if (cell.dataset.header) {
      values[cell.dataset.header] = cell.textContent || "";
    }
  });
  return values;
}

function renderFailedRowsMessage(failedRows) {
  if (!failedRows.length) {
    return;
  }

  const messages = failedRows.map((failure) => {
    const errors = (failure.errors || []).map((error) => error.message).join("; ");
    return `Строка ${failure.rowNumber}: ${errors}`;
  });
  fixError.textContent = `Не удалось исправить ${failedRows.length} строк(и): ${messages.join(" | ")}`;
}

function resetFixButton(clearStatus = true) {
  fixButton.disabled = false;
  fixButton.textContent = "✎ Применить исправления";
  if (clearStatus) {
    fixStatus.textContent = "";
  }
}
