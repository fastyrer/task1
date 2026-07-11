// ===== Вспомогательные функции =====
'use strict';

function formatWarning(warning) {
  const parts = [];
  if (warning.row) {
    parts.push(`строка ${warning.row}`);
  }
  if (warning.column) {
    parts.push(`колонка "${warning.column}"`);
  }
  parts.push(warning.message || "Предупреждение");
  return parts.join(": ");
}

function formatBytes(value) {
  if (value < 1024) {
    return `${value} Б`;
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} КБ`;
  }

  return `${(value / 1024 / 1024).toFixed(1)} МБ`;
}

function showError(message) {
  errorBlock.textContent = message;
}

function clearError() {
  errorBlock.textContent = "";
}

function setSearchEnabled(en) {
  searchInput.disabled = !en;
  searchButton.disabled = !en;
  clearSearchButton.disabled = !en;
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
