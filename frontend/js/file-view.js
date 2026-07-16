// file-view.js - отображение разобранного файла, статистики и ошибок строк.

import { appState } from "./state.js";
import { renderIcons } from "./icons.js";
import { formatBytes, formatWarning, showFilePanel, showWorkspaceView } from "./ui.js";

const headersBlock = document.getElementById("headersBlock");
const previewTable = document.getElementById("previewTable");
const fileInfo = document.getElementById("fileInfo");
const statsBlock = document.getElementById("statsBlock");
const warningsBlock = document.getElementById("warningsBlock");
const invalidRowsTable = document.getElementById("invalidRowsTable");
const fixRowRow = document.getElementById("fixRowRow");
const fixStatus = document.getElementById("fixStatus");

export function renderFileResult(payload) {
  renderFileInfo(payload);
  renderStats(payload.stats);
  renderHeaders(appState.currentHeaders);
  renderPreview(appState.currentHeaders, payload.previewRows || []);
  renderWarnings(payload.warnings || []);
  renderInvalidRows(payload.invalidRows || []);
}

export function renderStats(stats) {
  if (!stats) {
    statsBlock.className = "metrics-strip empty-state";
    statsBlock.textContent = "Сводка появится после загрузки файла.";
    return;
  }

  const metrics = [
    {
      label: "Строк", value: stats.rowCount, icon: "file-spreadsheet", tone: "neutral",
      panel: "preview", target: "previewTitle", action: "Открыть первые строки",
    },
    {
      label: "Колонок", value: stats.columnCount, icon: "columns-3", tone: "blue",
      panel: "preview", target: "headersTitle", action: "Открыть колонки файла",
    },
    {
      label: "Валидных", value: stats.validRowCount, icon: "circle-check-big", tone: "success",
      view: "contacts", action: "Перейти к сохранению контактов",
    },
    {
      label: "С ошибками", value: stats.invalidRowCount, icon: "triangle-alert", tone: "danger",
      panel: "issues", target: "invalidTitle", action: "Открыть строки с ошибками",
    },
    {
      label: "Пустых", value: stats.emptyRowCount, icon: "list-checks", tone: "neutral",
      panel: "issues", target: "warningsTitle", action: "Открыть диагностику файла",
    },
    {
      label: "Предупреждений", value: stats.warningCount, icon: "triangle-alert", tone: "warning",
      panel: "issues", target: "warningsTitle", action: "Открыть предупреждения",
    },
  ];

  const items = document.createDocumentFragment();
  metrics.forEach((metric) => {
    const item = document.createElement("button");
    item.type = "button";
    item.className = "metric";
    item.dataset.tone = metric.tone;
    item.title = metric.action;
    item.setAttribute("aria-label", `${metric.label}: ${metric.value || 0}. ${metric.action}`);
    item.addEventListener("click", () => openMetric(metric));

    const icon = document.createElement("span");
    icon.className = "metric-icon";
    icon.dataset.icon = metric.icon;
    icon.setAttribute("aria-hidden", "true");

    const caption = document.createElement("p");
    caption.className = "metric-label";
    caption.textContent = metric.label;

    const number = document.createElement("p");
    number.className = "metric-value";
    number.textContent = Number.isFinite(metric.value) ? metric.value : 0;

    item.append(icon, caption, number);
    items.appendChild(item);
  });

  statsBlock.className = "metrics-strip";
  statsBlock.replaceChildren(items);
  renderIcons(statsBlock);
}

function openMetric(metric) {
  showWorkspaceView(metric.view || "files");
  if (metric.panel) {
    showFilePanel(metric.panel);
  }
  if (metric.target) {
    window.requestAnimationFrame(() => {
      document.getElementById(metric.target)?.scrollIntoView({ behavior: "smooth", block: "start" });
    });
  }
}

export function renderWarnings(warnings) {
  if (!warnings.length) {
    warningsBlock.className = "empty-state";
    warningsBlock.textContent = "Предупреждений нет.";
    return;
  }

  const list = document.createElement("ul");
  list.className = "warning-list";
  warnings.forEach((warning) => {
    const item = document.createElement("li");
    item.textContent = formatWarning(warning);
    list.appendChild(item);
  });

  warningsBlock.className = "";
  warningsBlock.replaceChildren(list);
}

export function renderInvalidRows(rows) {
  if (!rows.length) {
    renderEmptyInvalidRows("Ошибочных строк нет.");
    return;
  }

  const columns = appState.currentHeaders.length
    ? appState.currentHeaders
    : Object.keys(rows[0].values || {});

  const thead = document.createElement("thead");
  const headerRow = document.createElement("tr");
  const rowNumHeader = document.createElement("th");
  rowNumHeader.textContent = "№";
  headerRow.appendChild(rowNumHeader);
  columns.forEach((header) => {
    const cell = document.createElement("th");
    cell.textContent = header;
    headerRow.appendChild(cell);
  });
  const errHeader = document.createElement("th");
  errHeader.textContent = "Ошибки";
  headerRow.appendChild(errHeader);
  thead.appendChild(headerRow);

  const tbody = document.createElement("tbody");
  rows.forEach((row) => {
    const tableRow = document.createElement("tr");
    tableRow.dataset.rowNumber = row.row || "";

    const numCell = document.createElement("td");
    numCell.className = "invalid-row-number";
    numCell.textContent = row.row || "";
    tableRow.appendChild(numCell);

    columns.forEach((header) => {
      const cell = document.createElement("td");
      cell.className = "invalid-value-cell";
      cell.contentEditable = "true";
      cell.dataset.header = header;
      cell.textContent = (row.values || {})[header] || "";
      tableRow.appendChild(cell);
    });

    const errCell = document.createElement("td");
    errCell.className = "invalid-errors";
    errCell.textContent = (row.errors || []).map(formatWarning).join("; ");
    tableRow.appendChild(errCell);

    tbody.appendChild(tableRow);
  });

  invalidRowsTable.replaceChildren(thead, tbody);
  fixRowRow.classList.remove("is-hidden");
  fixStatus.textContent = "";
  fixStatus.style.color = "";
}

export function renderEmptyInvalidRows(message) {
  renderEmptyTable(invalidRowsTable, message);
  fixRowRow.classList.add("is-hidden");
}

function renderFileInfo(payload) {
  const parts = [`fileId: ${payload.fileId}`];
  if (payload.originalFilename) {
    parts.push(`файл: ${payload.originalFilename}`);
  }
  if (payload.size) {
    parts.push(`размер: ${formatBytes(payload.size)}`);
  }
  if (payload.format) {
    parts.push(`формат: ${payload.format.toUpperCase()}`);
  }
  if (payload.encoding) {
    parts.push(`кодировка: ${payload.encoding}`);
  }
  if (payload.detectedMimeType) {
    parts.push(`MIME: ${payload.detectedMimeType}`);
  }
  if (payload.sheetName) {
    parts.push(`лист: ${payload.sheetName}`);
  }
  if (payload.headerRow) {
    parts.push(`заголовки: строка ${payload.headerRow}`);
  }

  fileInfo.textContent = parts.join(" · ");
}

function renderHeaders(headers) {
  if (!headers.length) {
    headersBlock.className = "empty-state";
    headersBlock.textContent = "Заголовки не найдены.";
    return;
  }

  headersBlock.className = "";
  const list = document.createElement("ul");
  list.className = "headers-list";
  headers.forEach((header) => {
    const item = document.createElement("li");
    item.textContent = header;
    list.appendChild(item);
  });
  headersBlock.replaceChildren(list);
}

function renderPreview(headers, rows) {
  if (!headers.length || !rows.length) {
    renderEmptyTable(previewTable, "Нет строк для preview.");
    return;
  }

  const thead = document.createElement("thead");
  const headerRow = document.createElement("tr");
  headers.forEach((header) => {
    const cell = document.createElement("th");
    cell.textContent = header;
    headerRow.appendChild(cell);
  });
  thead.appendChild(headerRow);

  const tbody = document.createElement("tbody");
  rows.forEach((row) => {
    const tableRow = document.createElement("tr");
    headers.forEach((header) => {
      const cell = document.createElement("td");
      cell.textContent = row[header] || "";
      tableRow.appendChild(cell);
    });
    tbody.appendChild(tableRow);
  });

  previewTable.replaceChildren(thead, tbody);
}

function renderEmptyTable(table, message) {
  const tbody = document.createElement("tbody");
  const row = document.createElement("tr");
  const cell = document.createElement("td");
  cell.className = "empty-state";
  cell.textContent = message;
  row.appendChild(cell);
  tbody.appendChild(row);
  table.replaceChildren(tbody);
}
