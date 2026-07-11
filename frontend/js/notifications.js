// ===== Уведомления =====
'use strict';

previewButton.addEventListener("click", handlePreview);
exportButton.addEventListener("click", handleExport);

// === Предпросмотр уведомлений ===
async function handlePreview() {
  clearNotificationError();

  const template = templateInput.textContent || "";
  if (!detectedPhoneColumn) {
    showNotificationError("Не удалось определить колонку с номером телефона.");
    return;
  }
  if (!template.trim()) {
    showNotificationError("Введите шаблон сообщения.");
    return;
  }

  previewButton.disabled = true;
  previewButton.textContent = "Загрузка...";

  try {
    const response = await fetch(previewURL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        fileId: currentFileId,
        phoneColumn: detectedPhoneColumn,
        template,
      }),
    });
    const payload = await response.json();

    if (!response.ok) {
      showNotificationError(payload.error || "Ошибка при формировании уведомлений.");
      return;
    }

    renderNotifications(payload);
  } catch (error) {
    showNotificationError("Не удалось подключиться к серверу.");
  } finally {
    previewButton.disabled = false;
    previewButton.textContent = "Предпросмотр";
  }
}

// === Уведомления ===
function renderNotifications(payload) {
  const notifications = payload.notifications || [];
  const skipped = payload.skipped || 0;
  if (!notifications.length) {
    showNotificationError("Нет строк с номерами телефонов.");
    notificationResults.style.display = "none";
    exportButton.style.display = "none";
    return;
  }

  notificationResults.style.display = "";
  exportButton.style.display = "";
  resultSummary.textContent = skipped > 0
    ? `Сформировано уведомлений: ${notifications.length}. Пропущено строк: ${skipped}`
    : `Сформировано уведомлений: ${notifications.length}`;

  const thead = document.createElement("thead");
  const headerRow = document.createElement("tr");
  ["№", "Телефон", "Сообщение"].forEach((header) => {
    const cell = document.createElement("th");
    cell.textContent = header;
    headerRow.appendChild(cell);
  });
  thead.appendChild(headerRow);

  const tbody = document.createElement("tbody");
  notifications.forEach((notification) => {
    const row = document.createElement("tr");
    if (!notification.phone) {
      row.className = "phone-empty";
    }

    [notification.row, notification.phone || "(пусто)", notification.text].forEach((value) => {
      const cell = document.createElement("td");
      cell.textContent = value;
      row.appendChild(cell);
    });
    tbody.appendChild(row);
  });

  notificationTable.replaceChildren(thead, tbody);
}

async function handleExport() {
  clearNotificationError();

  const template = templateInput.textContent || "";
  if (!detectedPhoneColumn) {
    showNotificationError("Не удалось определить колонку с номером телефона.");
    return;
  }
  if (!template.trim()) {
    showNotificationError("Введите шаблон сообщения.");
    return;
  }

  exportButton.disabled = true;
  exportButton.textContent = "Скачивание...";

  try {
    const response = await fetch(exportURL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        fileId: currentFileId,
        phoneColumn: detectedPhoneColumn,
        template,
      }),
    });

    if (!response.ok) {
      const payload = await response.json();
      showNotificationError(payload.error || "Ошибка при экспорте.");
      return;
    }

    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = "notifications.csv";
    document.body.appendChild(link);
    link.click();
    link.remove();
    URL.revokeObjectURL(url);
  } catch (error) {
    showNotificationError("Не удалось подключиться к серверу.");
  } finally {
    exportButton.disabled = false;
    exportButton.textContent = "Скачать CSV";
  }
}

function showNotificationError(message) {
  notificationError.textContent = message;
}

function clearNotificationError() {
  notificationError.textContent = "";
}
