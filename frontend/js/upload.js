// ===== Загрузка файла =====
'use strict';

uploadButton.addEventListener("click", async () => {
  clearError();
  setSearchEnabled(false);

  const file = fileInput.files[0];
  if (!file) {
    showError("Выберите CSV, XLS или XLSX-файл.");
    setSearchEnabled(Boolean(currentFileId));
    return;
  }

  const formData = new FormData();
  formData.append("file", file);
  if (sheetInput.value.trim()) {
    formData.append("sheet", sheetInput.value.trim());
  }

  uploadButton.disabled = true;
  uploadButton.textContent = "Загрузка...";

  try {
    const response = await fetch(uploadURL, {
      method: "POST",
      body: formData,
    });
    const payload = await response.json();

    if (!response.ok) {
      showError(payload.error || "Не удалось загрузить файл.");
      setSearchEnabled(Boolean(currentFileId));
      return;
    }

    renderResult(payload);
  } catch (error) {
    showError("Не удалось подключиться к серверу.");
    setSearchEnabled(Boolean(currentFileId));
  } finally {
    uploadButton.disabled = false;
    uploadButton.textContent = "Загрузить файл";
  }
});

function renderResult(payload) {
  currentFileId = payload.fileId || "";
  currentHeaders = payload.headers || [];
  detectedPhoneColumn = payload.detectedPhoneColumn || "";
  searchSequence += 1;
  searchInput.value = "";
  setSearchEnabled(Boolean(currentFileId));
  renderEmptySearch("Введите строку поиска.");
  renderFileInfo(payload);
  renderStats(payload.stats);
  renderHeaders(currentHeaders);
  renderPreview(currentHeaders, payload.previewRows || []);
  renderWarnings(payload.warnings || []);
  renderInvalidRows(payload.invalidRows || []);
  initConstructor();
  showSavePanel();
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

function renderStats(stats) {
  if (!stats) {
    statsBlock.className = "empty-state";
    statsBlock.textContent = "Сводка появится после загрузки файла.";
    return;
  }

  const metrics = [
    ["Строк", stats.rowCount],
    ["Колонок", stats.columnCount],
    ["Валидных", stats.validRowCount],
    ["С ошибками", stats.invalidRowCount],
    ["Пустых", stats.emptyRowCount],
    ["Предупреждений", stats.warningCount],
  ];

  const grid = document.createElement("div");
  grid.className = "summary-grid";
  metrics.forEach(([label, value]) => {
    const item = document.createElement("div");
    item.className = "metric";

    const caption = document.createElement("p");
    caption.className = "metric-label";
    caption.textContent = label;

    const number = document.createElement("p");
    number.className = "metric-value";
    number.textContent = Number.isFinite(value) ? value : 0;

    item.append(caption, number);
    grid.appendChild(item);
  });

  statsBlock.className = "";
  statsBlock.replaceChildren(grid);
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
    previewTable.innerHTML = '<tbody><tr><td class="empty-state">Нет строк для preview.</td></tr></tbody>';
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
    const tr = document.createElement("tr");
    headers.forEach((header) => {
      const cell = document.createElement("td");
      cell.textContent = row[header] || "";
      tr.appendChild(cell);
    });
    tbody.appendChild(tr);
  });

  previewTable.replaceChildren(thead, tbody);
}

function renderWarnings(warnings) {
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
