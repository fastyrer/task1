// search.js - поиск по строкам текущего файла и подсветка совпадений.

import { API, postJSON } from "./api.js";
import { setButtonLabel } from "./icons.js";
import { appState } from "./state.js";
import { clearError, showError } from "./ui.js";

const searchInput = document.getElementById("searchInput");
const searchButton = document.getElementById("searchButton");
const clearSearchButton = document.getElementById("clearSearchButton");
const searchInfo = document.getElementById("searchInfo");
const searchTable = document.getElementById("searchTable");

let searchTimer = 0;
let searchSequence = 0;

export function initSearch() {
  searchButton.addEventListener("click", runSearch);

  searchInput.addEventListener("input", () => {
    clearTimeout(searchTimer);
    if (!searchInput.value.trim()) {
      searchSequence += 1;
      renderEmptySearch("Введите строку поиска.");
      return;
    }
    searchTimer = window.setTimeout(runSearch, 300);
  });

  searchInput.addEventListener("keydown", (event) => {
    if (event.key === "Enter") {
      event.preventDefault();
      clearTimeout(searchTimer);
      runSearch();
    }
  });

  clearSearchButton.addEventListener("click", () => {
    clearTimeout(searchTimer);
    searchSequence += 1;
    searchInput.value = "";
    renderEmptySearch("Введите строку поиска.");
    searchInput.focus();
  });

  return {
    setEnabled,
    reset() {
      searchSequence += 1;
      searchInput.value = "";
      setEnabled(Boolean(appState.currentFileId));
      renderEmptySearch("Введите строку поиска.");
    },
  };
}

async function runSearch() {
  clearError();

  const query = searchInput.value.trim();
  if (!appState.currentFileId) {
    renderEmptySearch("Сначала загрузите файл.");
    return;
  }
  if (!query) {
    renderEmptySearch("Введите строку поиска.");
    return;
  }

  const sequence = searchSequence + 1;
  searchSequence = sequence;
  searchButton.disabled = true;
  setButtonLabel(searchButton, "Поиск...");
  searchInfo.textContent = "Идет поиск...";

  try {
    const { response, data } = await postJSON(API.search, {
      fileId: appState.currentFileId,
      query,
    });

    if (sequence !== searchSequence) {
      return;
    }
    if (!response.ok) {
      showError(data.error || "Не удалось выполнить поиск.");
      renderEmptySearch("Поиск не выполнен.");
      return;
    }

    renderSearchResults(data);
  } catch (error) {
    if (sequence === searchSequence) {
      showError("Не удалось подключиться к серверу.");
      renderEmptySearch("Поиск не выполнен.");
    }
  } finally {
    if (sequence === searchSequence) {
      searchButton.disabled = !appState.currentFileId;
      setButtonLabel(searchButton, "Найти");
    }
  }
}

function renderSearchResults(payload) {
  const rows = payload.rows || [];
  const headers = payload.headers || appState.currentHeaders;
  const query = payload.query || searchInput.value.trim();

  if (!rows.length) {
    searchInfo.textContent = "Совпадений не найдено.";
    renderEmptySearch("Нет строк с таким фрагментом.");
    return;
  }

  const total = Number.isFinite(payload.totalMatches) ? payload.totalMatches : rows.length;
  const returned = Number.isFinite(payload.returned) ? payload.returned : rows.length;
  const suffix = payload.truncated ? ` Показано ${returned} из ${total}.` : "";
  searchInfo.textContent = `Найдено строк: ${total}.${suffix}`;

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
      appendHighlightedText(cell, (row.values || {})[header] || "", query);
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
  if (!needle) {
    parent.textContent = text;
    return;
  }

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

function setEnabled(enabled) {
  searchInput.disabled = !enabled;
  searchButton.disabled = !enabled;
  clearSearchButton.disabled = !enabled;
}
