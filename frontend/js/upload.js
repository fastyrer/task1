// upload.js - выбор файла, отправка multipart-запроса и обновление экрана.

import { API, postForm } from "./api.js";
import { renderFileResult } from "./file-view.js";
import { appState, updateFileState } from "./state.js";
import { clearError, showError } from "./ui.js";

const fileInput = document.getElementById("fileInput");
const sheetInput = document.getElementById("sheetInput");
const uploadButton = document.getElementById("uploadButton");

export function initUpload({ searchController, contactsController }) {
  uploadButton.addEventListener("click", async () => {
    clearError();
    searchController.setEnabled(false);

    const file = fileInput.files[0];
    if (!file) {
      showError("Выберите CSV, XLS или XLSX-файл.");
      searchController.setEnabled(Boolean(appState.currentFileId));
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
      const { response, data } = await postForm(API.upload, formData);
      if (!response.ok) {
        showError(data.error || "Не удалось загрузить файл.");
        searchController.setEnabled(Boolean(appState.currentFileId));
        return;
      }

      updateFileState(data);
      searchController.reset();
      renderFileResult(data);
      contactsController.showSavePanel();
    } catch (error) {
      showError("Не удалось подключиться к серверу.");
      searchController.setEnabled(Boolean(appState.currentFileId));
    } finally {
      uploadButton.disabled = false;
      uploadButton.textContent = "Загрузить файл";
    }
  });
}
