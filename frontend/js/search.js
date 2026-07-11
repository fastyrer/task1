// ===== Поиск по данным =====
'use strict';

// === Поиск ===
searchButton.addEventListener("click", () => {
  runSearch();
});

searchInput.addEventListener("input", () => {
  clearTimeout(searchTimer);
  if (!searchInput.value.trim()) {
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

// === Поиск ===
async function runSearch() {
  clearError();

  const query = searchInput.value.trim();
  if (!currentFileId) {
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
  searchButton.textContent = "Поиск...";
  searchInfo.textContent = "Идет поиск...";

  try {
    const response = await fetch(searchURL, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        fileId: currentFileId,
        query,
      }),
    });
    const payload = await response.json();

    if (sequence !== searchSequence) {
      return;
    }

    if (!response.ok) {
      showError(payload.error || "Не удалось выполнить поиск.");
      renderEmptySearch("Поиск не выполнен.");
      return;
    }

    renderSearchResults(payload);
  } catch (error) {
    if (sequence === searchSequence) {
      showError("Не удалось подключиться к серверу.");
      renderEmptySearch("Поиск не выполнен.");
    }
  } finally {
    if (sequence === searchSequence) {
      searchButton.disabled = !currentFileId;
      searchButton.textContent = "Найти";
    }
  }
}

function renderSearchResults(payload) {
  const rows = payload.rows || [];
  const headers = payload.headers || currentHeaders;
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
    const tr = document.createElement("tr");

    const rowNumberCell = document.createElement("td");
    rowNumberCell.textContent = row.row || "";
    tr.appendChild(rowNumberCell);

    headers.forEach((header) => {
      const cell = document.createElement("td");
      appendHighlightedText(cell, (row.values || {})[header] || "", query);
      tr.appendChild(cell);
    });

    tbody.appendChild(tr);
  });

  searchTable.replaceChildren(thead, tbody);
}

function renderEmptySearch(message) {
  searchInfo.textContent = message;
  searchTable.innerHTML = '<tbody><tr><td class="empty-state">Результаты появятся после поиска.</td></tr></tbody>';
}
