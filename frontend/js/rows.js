// ===== Редактирование строк =====
'use strict';

// === Редактирование строк ===
fixButton.addEventListener("click", async () => {
  fixErrorEl.textContent = "";
  const tbody = invalidRowsTable.querySelector("tbody");
  if (!tbody) return;

  const rows = [];
  const trs = tbody.querySelectorAll("tr");
  trs.forEach((tr) => {
    const rowNumber = parseInt(tr.dataset.rowNumber, 10);
    if (!rowNumber) return;

    const values = {};
    const cells = tr.querySelectorAll("td[contenteditable]");
    cells.forEach((cell) => {
      const header = cell.dataset.header;
      if (header) {
        values[header] = cell.textContent || "";
      }
    });

    rows.push({ rowNumber, values });
  });

  if (!rows.length) {
    fixErrorEl.textContent = "Нет строк для исправления.";
    return;
  }

  fixButton.disabled = true;
  fixButton.textContent = "Сохранение...";
  fixStatusEl.textContent = "Идет проверка...";

  try {
    const response = await fetch(fixURL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ fileId: currentFileId, rows }),
    });
    const payload = await response.json();

    if (!response.ok) {
      fixErrorEl.textContent = payload.error || "Ошибка при сохранении.";
      fixButton.disabled = false;
      fixButton.textContent = "✎ Применить исправления";
      fixStatusEl.textContent = "";
      return;
    }

    if (payload.failed && payload.failed.length > 0) {
      const failedMsgs = payload.failed.map(f =>
        `Строка ${f.rowNumber}: ${(f.errors || []).map(e => e.message).join("; ")}`
      ).join(" | ");
      fixErrorEl.textContent = `Не удалось исправить ${payload.failed.length} строк(и): ${failedMsgs}`;
    }

    const fixed = payload.fixed || 0;
    const failed = (payload.failed || []).length;
    if (fixed > 0) {
      fixStatusEl.textContent = `✓ Исправлено: ${fixed}. ${failed > 0 ? `Ошибок: ${failed}` : ""}`;
      fixStatusEl.style.color = "#0f766e";

      const editedRows = trs.length;
      const remainingRows = [];
      for (const tr of trs) {
        const rn = parseInt(tr.dataset.rowNumber, 10);
        const isFailed = payload.failed && payload.failed.some(f => f.rowNumber === rn);
        if (isFailed) {
          remainingRows.push({
            row: rn,
            values: (() => {
              const v = {};
              tr.querySelectorAll("td[contenteditable]").forEach(c => {
                v[c.dataset.header] = c.textContent || "";
              });
              return v;
            })(),
            errors: payload.failed.find(f => f.rowNumber === rn).errors,
          });
        }
      }

      if (remainingRows.length > 0) {
        renderInvalidRows(remainingRows);
      } else {
        invalidRowsTable.innerHTML = '<tbody><tr><td class="empty-state">Все строки исправлены ✓</td></tr></tbody>';
        document.getElementById("fixRowRow").style.display = "none";
      }
    }

    fixButton.disabled = false;
    fixButton.textContent = "✎ Применить исправления";
  } catch (error) {
    fixErrorEl.textContent = "Не удалось подключиться к серверу.";
    fixButton.disabled = false;
    fixButton.textContent = "✎ Применить исправления";
    fixStatusEl.textContent = "";
  }
});

function renderInvalidRows(rows) {
  const fixRowRow = document.getElementById("fixRowRow");
  const fixButton = document.getElementById("fixRowsButton");
  const fixStatus = document.getElementById("fixStatus");

  if (!rows.length) {
    invalidRowsTable.innerHTML = '<tbody><tr><td class="empty-state">Ошибочных строк нет.</td></tr></tbody>';
    if (fixRowRow) fixRowRow.style.display = "none";
    return;
  }

  const columns = currentHeaders.length ? currentHeaders : Object.keys(rows[0].values || {});

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
    const tr = document.createElement("tr");
    tr.dataset.rowNumber = row.row || "";

    const numCell = document.createElement("td");
    numCell.textContent = row.row || "";
    numCell.style.fontWeight = "600";
    tr.appendChild(numCell);

    columns.forEach((header) => {
      const cell = document.createElement("td");
      cell.contentEditable = "true";
      cell.dataset.header = header;
      cell.textContent = (row.values || {})[header] || "";
      cell.style.cursor = "text";
      cell.style.background = "#fffdf5";
      cell.addEventListener("focus", () => {
        cell.style.outline = "2px solid var(--accent)";
      });
      cell.addEventListener("blur", () => {
        cell.style.outline = "none";
      });
      tr.appendChild(cell);
    });

    const errCell = document.createElement("td");
    errCell.style.color = "#a51d2d";
    errCell.style.fontSize = "13px";
    errCell.textContent = (row.errors || []).map(formatWarning).join("; ");
    tr.appendChild(errCell);

    tbody.appendChild(tr);
  });

  invalidRowsTable.replaceChildren(thead, tbody);
  if (fixRowRow) fixRowRow.style.display = "flex";
  if (fixStatus) fixStatus.textContent = "";
}
