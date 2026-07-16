// api.js - единая точка формирования URL и выполнения HTTP-запросов.

function apiPath(path) {
  if (window.location.protocol === "file:") {
    return `http://localhost:8080${path}`;
  }

  return path;
}

export const API = Object.freeze({
  upload: apiPath("/api/upload"),
  validateImport: apiPath("/api/imports/validate"),
  previewImport: apiPath("/api/imports/preview"),
  commitImport: apiPath("/api/imports/commit"),
  contacts: apiPath("/api/contacts"),
  preview: apiPath("/api/preview"),
  export: apiPath("/api/export"),
});

// getJSON выполняет read-only GET для ленивой загрузки справочника.
export async function getJSON(url) {
  const response = await fetch(url, { headers: { Accept: "application/json" } });
  const data = await response.json();
  return { response, data };
}

// postJSON выполняет POST и возвращает HTTP-ответ вместе с разобранным JSON.
export async function postJSON(url, payload) {
  const response = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const data = await response.json();
  return { response, data };
}

// putJSON изменяет одну существующую сущность и возвращает актуальную версию.
export async function putJSON(url, payload) {
  const response = await fetch(url, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  const data = await response.json();
  return { response, data };
}

// postForm отправляет multipart/form-data без ручной установки Content-Type.
export async function postForm(url, formData) {
  const response = await fetch(url, {
    method: "POST",
    body: formData,
  });
  const data = await response.json();
  return { response, data };
}

// postDownload возвращает сырой ответ, поскольку успешное тело является файлом.
export function postDownload(url, payload) {
  return fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
}
