// contacts.js - поиск, пагинация и ручное редактирование справочника PostgreSQL.

import { API, getJSON, putJSON } from "./api.js";
import { renderIcons } from "./icons.js";

const contactsTable = document.getElementById("contactsTable");
const contactsCountLabel = document.getElementById("contactsCountLabel");
const contactsCountInfo = document.getElementById("contactsCountInfo");
const contactsLoadStatus = document.getElementById("contactsLoadStatus");
const contactsError = document.getElementById("contactsError");
const contactsPageInfo = document.getElementById("contactsPageInfo");
const previousPageButton = document.getElementById("previousContactsPage");
const nextPageButton = document.getElementById("nextContactsPage");
const refreshContactsButton = document.getElementById("refreshContactsButton");
const contactsNavBadge = document.getElementById("contactsNavBadge");
const contactsSearchForm = document.getElementById("contactsSearchForm");
const contactsSearchInput = document.getElementById("contactsSearchInput");
const contactsSearchButton = document.getElementById("contactsSearchButton");
const clearContactsSearchButton = document.getElementById("clearContactsSearchButton");

const dateTimeFormat = new Intl.DateTimeFormat("ru-RU", {
  dateStyle: "short",
  timeStyle: "short",
});

let currentPage = 1;
let totalPages = 0;
let currentQuery = "";
let currentContacts = [];
let editingUID = "";
let savingUID = "";
let loaded = false;
let requestSequence = 0;

export function initContacts() {
  previousPageButton.addEventListener("click", () => requestPage(currentPage - 1));
  nextPageButton.addEventListener("click", () => requestPage(currentPage + 1));
  refreshContactsButton.addEventListener("click", () => requestPage(currentPage));

  contactsSearchForm.addEventListener("submit", (event) => {
    event.preventDefault();
    if (!confirmDiscardEdit()) {
      return;
    }
    currentQuery = contactsSearchInput.value.trim();
    loadContacts(1);
  });
  contactsSearchInput.addEventListener("input", updateSearchControls);
  clearContactsSearchButton.addEventListener("click", () => {
    if (!confirmDiscardEdit()) {
      return;
    }
    contactsSearchInput.value = "";
    currentQuery = "";
    updateSearchControls();
    loadContacts(1);
    contactsSearchInput.focus();
  });

  // Первая страница загружается только при первом открытии вкладки.
  document.addEventListener("workspace:view", (event) => {
    if (event.detail?.view === "contacts" && !loaded) {
      loadContacts(1);
    }
  });

  // После commit обновляется справочник; фильтр пользователя сохраняется.
  document.addEventListener("import:committed", () => {
    editingUID = "";
    loadContacts(loaded ? currentPage : 1);
  });
}

function requestPage(page) {
  if (confirmDiscardEdit()) {
    loadContacts(page);
  }
}

function confirmDiscardEdit() {
  if (!editingUID || window.confirm("Отменить несохранённые изменения контакта?")) {
    editingUID = "";
    return true;
  }
  return false;
}

async function loadContacts(page) {
  if (!Number.isInteger(page) || page < 1 || (totalPages > 0 && page > totalPages)) {
    return;
  }

  const requestID = ++requestSequence;
  setLoading(true);
  contactsError.textContent = "";
  contactsLoadStatus.textContent = `Загружаем страницу ${page}...`;
  const params = new URLSearchParams({ page: String(page) });
  if (currentQuery) {
    params.set("q", currentQuery);
  }

  try {
    const { response, data } = await getJSON(`${API.contacts}?${params.toString()}`);
    if (requestID !== requestSequence) {
      return;
    }
    if (!response.ok) {
      contactsError.textContent = data.error || "Не удалось загрузить контакты.";
      contactsLoadStatus.textContent = "Данные не обновлены";
      return;
    }

    loaded = true;
    currentPage = data.page || page;
    totalPages = data.totalPages || 0;
    currentQuery = data.query || "";
    currentContacts = data.items || [];
    editingUID = "";
    savingUID = "";
    contactsSearchInput.value = currentQuery;
    renderContacts();
    renderPagination(data);
    updateSearchControls();
  } catch (error) {
    if (requestID === requestSequence) {
      contactsError.textContent = "Не удалось подключиться к серверу.";
      contactsLoadStatus.textContent = "Данные не обновлены";
    }
  } finally {
    if (requestID === requestSequence) {
      setLoading(false);
    }
  }
}

function renderContacts() {
  if (!currentContacts.length) {
    renderEmptyContacts();
    return;
  }

  const thead = document.createElement("thead");
  const headerRow = document.createElement("tr");
  ["Телефон", "ФИО", "Email", "Скидка", "UID", "Обновлён", "Действия"].forEach((title) => {
    const cell = document.createElement("th");
    cell.textContent = title;
    headerRow.appendChild(cell);
  });
  thead.appendChild(headerRow);

  const tbody = document.createElement("tbody");
  currentContacts.forEach((contact) => {
    tbody.appendChild(editingUID === contact.uid ? createEditRow(contact) : createReadRow(contact));
  });
  contactsTable.replaceChildren(thead, tbody);
  renderIcons(contactsTable);
}

function createReadRow(contact) {
  const row = document.createElement("tr");
  row.append(
    contactCell(contact.phone, "contact-phone"),
    contactCell(contact.name),
    contactCell(contact.email),
    contactCell(contact.discount),
    contactCell(contact.uid, "contact-uid"),
    contactCell(formatDate(contact.updatedAt), "contact-date"),
    contactActionsCell(createIconButton("pencil", "Изменить контакт", "contact-edit-button", () => startEditing(contact.uid))),
  );
  return row;
}

function createEditRow(contact) {
  const row = document.createElement("tr");
  row.className = "contact-edit-row";
  row.dataset.contactUid = contact.uid;
  row.append(
    contactInputCell("phone", contact.phone, "Телефон", "tel"),
    contactInputCell("name", contact.name, "ФИО"),
    contactInputCell("email", contact.email, "Email", "email"),
    contactInputCell("discount", contact.discount, "Скидка", "decimal"),
    contactCell(contact.uid, "contact-uid"),
    contactCell(formatDate(contact.updatedAt), "contact-date"),
    contactActionsCell(
      createIconButton("save", "Сохранить в PostgreSQL", "contact-save-button", () => saveContact(contact, row)),
      createIconButton("x", "Отменить изменения", "contact-cancel-button", cancelEditing),
    ),
  );
  return row;
}

function contactCell(value, className = "") {
  const cell = document.createElement("td");
  if (className) {
    cell.className = className;
  }
  cell.textContent = value || "(не указано)";
  return cell;
}

function contactInputCell(field, value, label, inputMode = "text") {
  const cell = document.createElement("td");
  const input = document.createElement("input");
  input.type = "text";
  input.className = "contact-edit-input";
  input.dataset.contactField = field;
  input.value = value || "";
  input.inputMode = inputMode;
  input.setAttribute("aria-label", label);
  if (field === "phone") {
    input.required = true;
  }
  cell.appendChild(input);
  return cell;
}

function contactActionsCell(...buttons) {
  const cell = document.createElement("td");
  cell.className = "contact-actions-cell";
  const actions = document.createElement("div");
  actions.className = "action-row contact-row-actions";
  actions.append(...buttons);
  cell.appendChild(actions);
  return cell;
}

function createIconButton(icon, label, className, onClick) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = `icon-button ${className}`;
  button.title = label;
  button.setAttribute("aria-label", label);
  const iconNode = document.createElement("span");
  iconNode.dataset.icon = icon;
  iconNode.setAttribute("aria-hidden", "true");
  button.appendChild(iconNode);
  button.addEventListener("click", onClick);
  return button;
}

function startEditing(uid) {
  if (savingUID || editingUID && editingUID !== uid && !window.confirm("Отменить несохранённые изменения контакта?")) {
    return;
  }
  contactsError.textContent = "";
  editingUID = uid;
  renderContacts();
  contactsTable.querySelector(`[data-contact-uid="${uid}"] [data-contact-field="phone"]`)?.focus();
}

function cancelEditing() {
  editingUID = "";
  contactsError.textContent = "";
  contactsLoadStatus.textContent = "Изменения отменены";
  renderContacts();
}

async function saveContact(original, row) {
  if (savingUID) {
    return;
  }
  const payload = {
    phone: contactFieldValue(row, "phone"),
    name: contactFieldValue(row, "name"),
    email: contactFieldValue(row, "email"),
    discount: contactFieldValue(row, "discount"),
    version: original.updatedAt,
  };
  if (!payload.phone.trim()) {
    contactsError.textContent = "Телефон является обязательным полем.";
    row.querySelector('[data-contact-field="phone"]')?.focus();
    return;
  }

  savingUID = original.uid;
  contactsError.textContent = "";
  contactsLoadStatus.textContent = "Сохраняем контакт в PostgreSQL...";
  setEditRowDisabled(row, true);
  try {
    const { response, data } = await putJSON(`${API.contacts}/${encodeURIComponent(original.uid)}`, payload);
    if (!response.ok) {
      contactsError.textContent = data.error || "Не удалось сохранить изменения контакта.";
      contactsLoadStatus.textContent = response.status === 409
        ? "Сохранение остановлено из-за конфликта данных"
        : "Изменения не сохранены";
      return;
    }

    currentContacts = currentContacts.map((contact) => contact.uid === data.uid ? data : contact);
    editingUID = "";
    contactsLoadStatus.textContent = `Контакт ${data.phone} сохранён в PostgreSQL`;
    renderContacts();
  } catch (error) {
    contactsError.textContent = "Не удалось подключиться к серверу.";
    contactsLoadStatus.textContent = "Изменения не сохранены";
  } finally {
    savingUID = "";
    const activeRow = contactsTable.querySelector(`[data-contact-uid="${original.uid}"]`);
    if (activeRow) {
      setEditRowDisabled(activeRow, false);
    }
  }
}

function contactFieldValue(row, field) {
  return row.querySelector(`[data-contact-field="${field}"]`)?.value || "";
}

function setEditRowDisabled(row, disabled) {
  row.setAttribute("aria-busy", String(disabled));
  row.querySelectorAll("input, button").forEach((control) => {
    control.disabled = disabled;
  });
}

function formatDate(value) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "(не указано)" : dateTimeFormat.format(date);
}

function renderEmptyContacts() {
  const tbody = document.createElement("tbody");
  const row = document.createElement("tr");
  const cell = document.createElement("td");
  cell.className = "empty-state";
  cell.textContent = currentQuery
    ? "По вашему запросу контакты не найдены."
    : "В PostgreSQL пока нет подтверждённых контактов.";
  row.appendChild(cell);
  tbody.appendChild(row);
  contactsTable.replaceChildren(tbody);
}

function renderPagination(data) {
  const total = Number(data.total) || 0;
  contactsCountLabel.textContent = currentQuery ? "Найдено в PostgreSQL" : "Записей в PostgreSQL";
  contactsCountInfo.textContent = total.toLocaleString("ru-RU");
  contactsLoadStatus.textContent = total > 0
    ? `Показано записей: ${(data.items || []).length}`
    : currentQuery ? "Совпадений нет" : "Справочник пуст";
  contactsPageInfo.textContent = totalPages > 0
    ? `Страница ${currentPage} из ${totalPages}`
    : "Страница 0 из 0";
  previousPageButton.disabled = currentPage <= 1;
  nextPageButton.disabled = totalPages === 0 || currentPage >= totalPages;

  // Бейдж навигации показывает полный справочник, поэтому фильтр его не перезаписывает.
  if (!currentQuery) {
    contactsNavBadge.textContent = total.toLocaleString("ru-RU");
    contactsNavBadge.classList.remove("is-hidden");
  }
}

function updateSearchControls() {
  clearContactsSearchButton.disabled = !contactsSearchInput.value && !currentQuery;
}

function setLoading(loading) {
  refreshContactsButton.disabled = loading;
  contactsSearchButton.disabled = loading;
  clearContactsSearchButton.disabled = loading || !contactsSearchInput.value && !currentQuery;
  previousPageButton.disabled = loading || currentPage <= 1;
  nextPageButton.disabled = loading || totalPages === 0 || currentPage >= totalPages;
  contactsTable.setAttribute("aria-busy", String(loading));
}
