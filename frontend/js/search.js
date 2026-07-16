// search.js - локальный поиск по строкам черновика без обращения к PostgreSQL.

import { appState, currentDraft } from "./state.js";
import { clearError } from "./ui.js";

const searchInput = document.getElementById("searchInput");
const searchButton = document.getElementById("searchButton");
const clearSearchButton = document.getElementById("clearSearchButton");
const searchInfo = document.getElementById("searchInfo");
const searchTable = document.getElementById("searchTable");
const localSearchLimit = 1000;

let searchTimer = 0;

export function initSearch() {
  searchButton.addEventListener("click", runSearch);
  searchInput.addEventListener("input", () => {
    window.clearTimeout(searchTimer);
    if (!searchInput.value.trim()) {
      renderEmptySearch("Введите строку поиска.");
      return;
    }
    searchTimer = window.setTimeout(runSearch, 250);
  });
  searchInput.addEventListener("keydown", (event) => {
    if (event.key === "Enter") {
      event.preventDefault();
      window.clearTimeout(searchTimer);
      runSearch();
    }
  });
  clearSearchButton.addEventListener("click", () => {
    window.clearTimeout(searchTimer);
    searchInput.value = "";
    renderEmptySearch("Введите строку поиска.");
    searchInput.focus();
  });

  return { setEnabled, reset };
}

function runSearch() {
  clearError();
  const draft = currentDraft();
  const query = searchInput.value.trim();
  if (!draft) {
    renderEmptySearch("Сначала проверьте файл.");
    return;
  }
  if (!query) {
    renderEmptySearch("Введите строку поиска.");
    return;
  }

  const normalizedQuery = query.toLocaleLowerCase();
  const matchedRows = draft.rows.filter((row) => draft.headers.some((header) => (
    String(row.values?.[header] || "").toLocaleLowerCase().includes(normalizedQuery)
  )));
  const rows = matchedRows.slice(0, localSearchLimit).map((row) => ({
    row: row.rowNumber,
    values: row.values,
  }));

  renderSearchResults({
    query,
    headers: draft.headers,
    rows,
    totalMatches: matchedRows.length,
    returned: rows.length,
    truncated: matchedRows.length > rows.length,
  });
}

function renderSearchResults(payload) {
  const rows = payload.rows || [];
  const headers = payload.headers || appState.currentHeaders;
  const query = payload.query || searchInput.value.trim();

  if (!rows.length) {
    renderEmptySearch("Совпадений не найдено.");
    return;
  }

  const suffix = payload.truncated
    ? ` Показано ${payload.returned} из ${payload.totalMatches}.`
    : "";
  searchInfo.textContent = `Найдено строк: ${payload.totalMatches}.${suffix}`;

  const thead = document.createElement("thead");
  const headerRow = document.createElement("tr");
  ["Строка", ...headers].forEach((header) => {
    const cell = document.createElement("th");
    cell.textContent = header;
    headerRow.appendChild(cell);
  });
  thead.appendChild(headerRow);

  const tbody = document.createElement("tbody");
  rows.forEach((row) => {
    const tableRow = document.createElement("tr");
    const rowNumberCell = document.createElement("td");
    rowNumberCell.textContent = row.row || "";
    tableRow.appendChild(rowNumberCell);
    headers.forEach((header) => {
      const cell = document.createElement("td");
      appendHighlightedText(cell, row.values?.[header] || "", query);
      tableRow.appendChild(cell);
    });
    tbody.appendChild(tableRow);
  });
  searchTable.replaceChildren(thead, tbody);
}

function renderEmptySearch(message) {
  searchInfo.textContent = message;
  const tbody = document.createElement("tbody");
  const row = document.createElement("tr");
  const cell = document.createElement("td");
  cell.className = "empty-state";
  cell.textContent = "Результаты появятся после поиска.";
  row.appendChild(cell);
  tbody.appendChild(row);
  searchTable.replaceChildren(tbody);
}

function appendHighlightedText(parent, value, query) {
  const text = String(value || "");
  const needle = String(query || "").toLocaleLowerCase();
  const haystack = text.toLocaleLowerCase();
  let position = 0;

  while (position < text.length) {
    const index = haystack.indexOf(needle, position);
    if (index === -1) {
      parent.appendChild(document.createTextNode(text.slice(position)));
      break;
    }
    if (index > position) {
      parent.appendChild(document.createTextNode(text.slice(position, index)));
    }
    const marker = document.createElement("mark");
    marker.textContent = text.slice(index, index + query.length);
    parent.appendChild(marker);
    position = index + query.length;
  }
}

function reset() {
  searchInput.value = "";
  setEnabled(Boolean(currentDraft()));
  renderEmptySearch(currentDraft() ? "Введите строку поиска." : "Поиск доступен после проверки файла.");
}

function setEnabled(enabled) {
  searchInput.disabled = !enabled;
  searchButton.disabled = !enabled;
  clearSearchButton.disabled = !enabled;
}
