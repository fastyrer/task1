// upload.js - stateless-проверка файла и управление локальным черновиком.

import { API, postForm } from "./api.js";
import { loadActiveDraft, removeActiveDraft, saveActiveDraft } from "./draft-store.js";
import { renderFileResult, resetFileView } from "./file-view.js";
import { setButtonLabel } from "./icons.js";
import { appState, clearDraftState, currentDraft, setDraftState } from "./state.js";
import { clearError, showError, showFilePanel } from "./ui.js";

const fileInput = document.getElementById("fileInput");
const sheetInput = document.getElementById("sheetInput");
const uploadButton = document.getElementById("uploadButton");
const filePickerLabel = document.getElementById("filePickerLabel");
const discardDraftButtons = document.querySelectorAll("[data-discard-draft]");
const fileWorkflowFooter = document.getElementById("fileWorkflowFooter");
let draftSaveTimer = 0;

export async function initUpload({ searchController, importController }) {
  fileInput.addEventListener("change", () => {
    filePickerLabel.textContent = fileInput.files[0]?.name || "Выберите CSV, XLS или XLSX";
  });

  uploadButton.addEventListener("click", async () => {
    clearError();
    const file = fileInput.files[0];
    if (!file) {
      showError("Выберите CSV, XLS или XLSX-файл.");
      return;
    }
    if (currentDraft() && !window.confirm("Заменить текущий локальный черновик новым файлом?")) {
      return;
    }

    const formData = new FormData();
    formData.append("file", file);
    if (sheetInput.value.trim()) {
      formData.append("sheet", sheetInput.value.trim());
    }

    uploadButton.disabled = true;
    setButtonLabel(uploadButton, "Проверка...");
    try {
      const { response, data } = await postForm(API.upload, formData);
      if (!response.ok) {
        showError(data.error || "Не удалось проверить файл.");
        return;
      }

      await saveActiveDraft(data, false);
      activateDraft(data, searchController, importController);
    } catch (error) {
      showError("Не удалось проверить или сохранить локальный черновик.");
    } finally {
      uploadButton.disabled = false;
      setButtonLabel(uploadButton, "Проверить файл");
    }
  });

  // Кнопки в сводке и финальном этапе отменяют один и тот же локальный черновик.
  discardDraftButtons.forEach((button) => {
    button.addEventListener("click", async () => {
      if (!currentDraft() || !window.confirm("Удалить локальный черновик? PostgreSQL не изменится.")) {
        return;
      }
      await clearLocalDraft(searchController, importController, true);
    });
  });

  // После успешного commit backend уже сохранил данные, поэтому локальная копия удаляется без вопроса.
  document.addEventListener("import:committed", () => {
    clearLocalDraft(searchController, importController, false).catch(() => {
      showError("Импорт завершён, но локальный черновик не удалось удалить автоматически.");
    });
  });

  // Редактирование не обращается к серверу: изменения с небольшой задержкой остаются в IndexedDB.
  document.addEventListener("draft:edited", () => {
    window.clearTimeout(draftSaveTimer);
    draftSaveTimer = window.setTimeout(() => {
      if (appState.validation) {
        saveActiveDraft(appState.validation, true).catch(() => {
          showError("Не удалось сохранить локальные изменения в браузере.");
        });
      }
    }, 300);
  });

  try {
    const restored = await loadActiveDraft();
    if (restored?.validation) {
      activateDraft(restored.validation, searchController, importController, restored.dirty);
    }
  } catch (error) {
    showError("Локальный черновик недоступен. Проверьте настройки хранилища браузера.");
  }
}

function activateDraft(validation, searchController, importController, dirty = false) {
  setDraftState(validation, dirty);
  renderFileResult(validation);
  searchController.reset();
  importController.showImportPanel(validation);
  fileWorkflowFooter.classList.remove("is-hidden");
  showFilePanel("preview");
}

async function clearLocalDraft(searchController, importController, resetImport) {
  window.clearTimeout(draftSaveTimer);
  await removeActiveDraft();
  clearDraftState();
  resetFileView();
  searchController.reset();
  fileWorkflowFooter.classList.add("is-hidden");
  fileInput.value = "";
  sheetInput.value = "";
  filePickerLabel.textContent = "Выберите CSV, XLS или XLSX";
  if (resetImport) {
    importController.reset();
  }
}
