// notifications.js - предпросмотр и CSV-экспорт рассылки по всем контактам БД.

import { API, postDownload, postJSON } from "./api.js";
import { getTemplate } from "./template-editor.js";

const previewButton = document.getElementById("previewButton");
const exportButton = document.getElementById("exportButton");
const notificationError = document.getElementById("notificationError");
const notificationResults = document.getElementById("notificationResults");
const resultSummary = document.getElementById("resultSummary");
const notificationTable = document.getElementById("notificationTable");

export function initNotifications() {
  previewButton.addEventListener("click", handlePreview);
  exportButton.addEventListener("click", handleExport);
}

async function handlePreview() {
  clearNotificationError();

  const template = getTemplate();
  if (!template.trim()) {
    showNotificationError("Введите шаблон сообщения.");
    return;
  }

  previewButton.disabled = true;
  previewButton.textContent = "Загрузка...";

  try {
    const { response, data } = await postJSON(API.preview, { template });
    if (!response.ok) {
      showNotificationError(data.error || "Ошибка при формировании уведомлений.");
      return;
    }

    renderNotifications(data);
  } catch (error) {
    showNotificationError("Не удалось подключиться к серверу.");
  } finally {
    previewButton.disabled = false;
    previewButton.textContent = "Предпросмотр";
  }
}

function renderNotifications(payload) {
  const notifications = payload.notifications || [];
  const skipped = payload.skipped || 0;
  if (!notifications.length) {
    showNotificationError("В базе данных нет сохранённых контактов.");
    notificationResults.classList.add("is-hidden");
    exportButton.classList.add("is-hidden");
    return;
  }

  notificationResults.classList.remove("is-hidden");
  exportButton.classList.remove("is-hidden");
  resultSummary.textContent = skipped > 0
    ? `Сформировано уведомлений: ${notifications.length}. Пропущено контактов: ${skipped}`
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

  const template = getTemplate();
  if (!template.trim()) {
    showNotificationError("Введите шаблон сообщения.");
    return;
  }

  exportButton.disabled = true;
  exportButton.textContent = "Скачивание...";

  try {
    const response = await postDownload(API.export, { template });
    if (!response.ok) {
      const data = await response.json();
      showNotificationError(data.error || "Ошибка при экспорте.");
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
