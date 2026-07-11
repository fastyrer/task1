// ===== Контакты =====
'use strict';

// === Контакты (сохранение и конфликты) ===
saveButton.addEventListener("click", async () => {
  saveErrorEl.textContent = "";
  saveButton.disabled = true;
  saveButton.textContent = "Сохранение...";
  saveStatus.textContent = "Идет сохранение...";

  try {
    const response = await fetch(saveURL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ fileId: currentFileId }),
    });
    const payload = await response.json();

    if (!response.ok) {
      saveErrorEl.textContent = payload.error || "Ошибка при сохранении.";
      saveButton.disabled = false;
      saveButton.textContent = "💾 Сохранить данные в БД";
      saveStatus.textContent = "";
      return;
    }

    saveResult.style.display = "block";
    saveStatsEl.replaceChildren();

    const savedStat = document.createElement("div");
    savedStat.className = "save-stat ok";
    savedStat.textContent = `✓ Сохранено: ${payload.saved || 0}`;
    saveStatsEl.appendChild(savedStat);

    const skippedStat = document.createElement("div");
    skippedStat.className = payload.skipped > 0 ? "save-stat warn" : "save-stat ok";
    skippedStat.textContent = payload.skipped > 0 ? `⚠ Пропущено: ${payload.skipped}` : `✓ Пропущено: 0`;
    saveStatsEl.appendChild(skippedStat);

    const conflicts = payload.conflicts || [];
    if (conflicts.length > 0) {
      const conflictStat = document.createElement("div");
      conflictStat.className = "save-stat danger";
      conflictStat.textContent = `! Конфликтов: ${conflicts.length}`;
      saveStatsEl.appendChild(conflictStat);
    }

    saveButton.textContent = "💾 Сохранено";
    saveButton.disabled = true;
    saveStatus.textContent = "Данные сохранены";

    renderConflicts(conflicts);
  } catch (error) {
    saveErrorEl.textContent = "Не удалось подключиться к серверу.";
    saveButton.disabled = false;
    saveButton.textContent = "💾 Сохранить данные в БД";
    saveStatus.textContent = "";
  }
});

function showSavePanel() {
  saveSection.style.display = "block";
  saveResult.style.display = "none";
  saveErrorEl.textContent = "";
  saveStatus.textContent = "Нажмите, чтобы сохранить валидные данные в базу";
  saveButton.disabled = false;
  saveButton.textContent = "💾 Сохранить данные в БД";
  currentConflicts = [];
}

function renderConflicts(conflicts) {
  currentConflicts = conflicts || [];
  if (!conflicts.length) {
    conflictsSection.style.display = "none";
    return;
  }

  conflictsSection.style.display = "block";
  batchResolveRow.style.display = "flex";
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

    const thead = document.createElement("thead");
    const headerRow = document.createElement("tr");
    ["Поле", "Существующее", "Новое"].forEach((text) => {
      const th = document.createElement("th");
      th.textContent = text;
      headerRow.appendChild(th);
    });
    thead.appendChild(headerRow);
    table.appendChild(thead);

    const tbody = document.createElement("tbody");
    const allKeys = new Set([
      ...Object.keys(conflict.existing || {}),
      ...Object.keys(conflict.incoming || {}),
    ]);

    allKeys.forEach((key) => {
      if (key === "phone") return;
      const ev = (conflict.existing || {})[key] || "";
      const iv = (conflict.incoming || {})[key] || "";
      if (ev === "" && iv === "") return;

      const tr = document.createElement("tr");
      if (ev !== iv) {
        tr.className = "diff";
      }

      const keyCell = document.createElement("td");
      keyCell.textContent = key;
      tr.appendChild(keyCell);

      const existingCell = document.createElement("td");
      existingCell.textContent = ev || "(пусто)";
      tr.appendChild(existingCell);

      const incomingCell = document.createElement("td");
      incomingCell.textContent = iv || "(пусто)";
      tr.appendChild(incomingCell);

      tbody.appendChild(tr);
    });
    table.appendChild(tbody);
    card.appendChild(table);

    const actions = document.createElement("div");
    actions.className = "conflict-actions";

    const skipBtn = document.createElement("button");
    skipBtn.className = "btn-outline skip";
    skipBtn.textContent = "Пропустить";
    skipBtn.addEventListener("click", () => resolveConflict(conflict.phone, "skip", index));

    const replaceBtn = document.createElement("button");
    replaceBtn.className = "btn-outline replace";
    replaceBtn.textContent = "Заменить";
    replaceBtn.addEventListener("click", () => resolveConflict(conflict.phone, "replace", index));

    const mergeBtn = document.createElement("button");
    mergeBtn.className = "btn-outline merge";
    mergeBtn.textContent = "Объединить";
    mergeBtn.addEventListener("click", () => resolveConflict(conflict.phone, "merge", index));

    actions.append(skipBtn, replaceBtn, mergeBtn);
    card.appendChild(actions);
    conflictsBlock.appendChild(card);
  });
}

async function resolveConflict(phone, action, index) {
  try {
    const response = await fetch(resolveURL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        fileId: currentFileId,
        phone,
        action,
      }),
    });
    const payload = await response.json();
    if (!response.ok) {
      showError(payload.error || "Ошибка при разрешении конфликта.");
      return;
    }

    const card = conflictsBlock.querySelector(`[data-index="${index}"]`);
    if (card) {
      card.style.opacity = "0.5";
      card.querySelector(".conflict-actions").innerHTML =
        `<span style="color: #0f766e; font-weight: 600;">✓ ${getActionLabel(action)}</span>`;
    }

    const remaining = conflictsBlock.querySelectorAll(".conflict-card:not([style*='opacity: 0.5'])");
    if (remaining.length === 0) {
      batchResolveRow.style.display = "none";
    }
  } catch (error) {
    showError("Не удалось подключиться к серверу.");
  }
}

function getActionLabel(action) {
  switch (action) {
    case "skip": return "Пропущено";
    case "replace": return "Заменено";
    case "merge": return "Объединено";
    default: return action;
  }
}

resolveAllSkip.addEventListener("click", () => resolveAllConflicts("skip"));
resolveAllReplace.addEventListener("click", () => resolveAllConflicts("replace"));
resolveAllMerge.addEventListener("click", () => resolveAllConflicts("merge"));

async function resolveAllConflicts(action) {
  try {
    const response = await fetch(resolveAllURL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        fileId: currentFileId,
        action,
      }),
    });
    const payload = await response.json();
    if (!response.ok) {
      showError(payload.error || "Ошибка при массовом разрешении.");
      return;
    }

    const cards = conflictsBlock.querySelectorAll(".conflict-card");
    cards.forEach((card) => {
      card.style.opacity = "0.5";
      card.querySelector(".conflict-actions").innerHTML =
        `<span style="color: #0f766e; font-weight: 600;">✓ ${getActionLabel(action)}</span>`;
    });
    batchResolveRow.style.display = "none";
  } catch (error) {
    showError("Не удалось подключиться к серверу.");
  }
}
