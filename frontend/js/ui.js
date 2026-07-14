// ui.js - общие функции отображения, не привязанные к одному сценарию.

const errorBlock = document.getElementById("errorBlock");

export function initCollapsiblePanels() {
  document.querySelectorAll(".collapsible-header").forEach((header) => {
    header.addEventListener("click", () => {
      header.parentElement.classList.toggle("closed");
    });
  });
}

export function showError(message) {
  errorBlock.textContent = message;
}

export function clearError() {
  errorBlock.textContent = "";
}

export function formatWarning(warning) {
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

export function formatBytes(value) {
  if (value < 1024) {
    return `${value} Б`;
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} КБ`;
  }

  return `${(value / 1024 / 1024).toFixed(1)} МБ`;
}
