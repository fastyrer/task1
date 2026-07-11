// ===== DOM-ссылки и константы =====
'use strict';

function apiPath(path) {
  if (window.location.protocol === "file:") {
    return `http://localhost:8080${path}`;
  }

  return path;
}

const fileInput = document.getElementById("fileInput");
const sheetInput = document.getElementById("sheetInput");
const uploadButton = document.getElementById("uploadButton");
const errorBlock = document.getElementById("errorBlock");
const headersBlock = document.getElementById("headersBlock");
const previewTable = document.getElementById("previewTable");
const fileInfo = document.getElementById("fileInfo");
const statsBlock = document.getElementById("statsBlock");
const warningsBlock = document.getElementById("warningsBlock");
const invalidRowsTable = document.getElementById("invalidRowsTable");
const constructorSection = document.getElementById("constructorSection");
const templateInput = document.getElementById("templateInput");
const placeholderHint = document.getElementById("placeholderHint");
const constructorHeaders = document.getElementById("constructorHeaders");
const previewButton = document.getElementById("previewButton");
const exportButton = document.getElementById("exportButton");
const notificationError = document.getElementById("notificationError");
const notificationResults = document.getElementById("notificationResults");
const resultSummary = document.getElementById("resultSummary");
const notificationTable = document.getElementById("notificationTable");
const searchInput = document.getElementById("searchInput");
const searchButton = document.getElementById("searchButton");
const clearSearchButton = document.getElementById("clearSearchButton");
const searchInfo = document.getElementById("searchInfo");
const searchTable = document.getElementById("searchTable");
const saveSection = document.getElementById("saveSection");
const saveButton = document.getElementById("saveButton");
const saveStatus = document.getElementById("saveStatus");
const saveErrorEl = document.getElementById("saveError");
const saveResult = document.getElementById("saveResult");
const saveStatsEl = document.getElementById("saveStats");
const fixRowRow = document.getElementById("fixRowRow");
const fixButton = document.getElementById("fixRowsButton");
const fixStatusEl = document.getElementById("fixStatus");
const fixErrorEl = document.getElementById("fixError");
const conflictsSection = document.getElementById("conflictsSection");
const conflictsBlock = document.getElementById("conflictsBlock");
const batchResolveRow = document.getElementById("batchResolveRow");
const resolveAllSkip = document.getElementById("resolveAllSkip");
const resolveAllReplace = document.getElementById("resolveAllReplace");
const resolveAllMerge = document.getElementById("resolveAllMerge");

const uploadURL = apiPath("/api/upload");
const previewURL = apiPath("/api/preview");
const exportURL = apiPath("/api/export");
const searchURL = apiPath("/api/search");
const resolveURL = apiPath("/api/contacts/resolve");
const resolveAllURL = apiPath("/api/contacts/resolve-all");
const saveURL = apiPath("/api/contacts/save");
const fixURL = apiPath("/api/rows/fix");

// === Инициализация ===
document.querySelectorAll(".collapsible-header").forEach(header => {
  header.addEventListener("click", () => {
    header.parentElement.classList.toggle("closed");
  });
});
