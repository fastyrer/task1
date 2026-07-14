// state.js - минимальное общее состояние последнего загруженного файла.

export const appState = {
  currentFileId: "",
  currentHeaders: [],
};

// updateFileState обновляет значения, которыми пользуются поиск и контакты.
export function updateFileState(payload) {
  appState.currentFileId = payload.fileId || "";
  appState.currentHeaders = payload.headers || [];
}
