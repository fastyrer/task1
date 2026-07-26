// draft-store.js - локальное временное хранение одного черновика в IndexedDB.

const databaseName = "task1-local";
const storeName = "drafts";
const activeDraftKey = "active";
const databaseVersion = 1;
const draftLifetimeMs = 24 * 60 * 60 * 1000;

function openDatabase() {
  return new Promise((resolve, reject) => {
    if (!window.indexedDB) {
      reject(new Error("IndexedDB недоступна"));
      return;
    }

    const request = window.indexedDB.open(databaseName, databaseVersion);
    request.onupgradeneeded = () => {
      const database = request.result;
      if (!database.objectStoreNames.contains(storeName)) {
        database.createObjectStore(storeName, { keyPath: "id" });
      }
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error || new Error("Не удалось открыть IndexedDB"));
  });
}

function runTransaction(mode, operation) {
  return openDatabase().then((database) => new Promise((resolve, reject) => {
    const transaction = database.transaction(storeName, mode);
    const store = transaction.objectStore(storeName);
    const request = operation(store);
    let result;

    request.onsuccess = () => {
      result = request.result;
    };
    request.onerror = () => reject(request.error || new Error("Ошибка IndexedDB"));
    transaction.oncomplete = () => {
      database.close();
      resolve(result);
    };
    transaction.onabort = () => {
      database.close();
      reject(transaction.error || new Error("Транзакция IndexedDB отменена"));
    };
    transaction.onerror = () => {
      database.close();
      reject(transaction.error || new Error("Транзакция IndexedDB не выполнена"));
    };
  }));
}

// dirty сохраняется отдельно: после перезагрузки непроверенные правки нельзя считать валидными.
export function saveActiveDraft(validation, dirty = false) {
  return runTransaction("readwrite", (store) => store.put({
    id: activeDraftKey,
    savedAt: Date.now(),
    dirty,
    validation,
  }));
}

export async function loadActiveDraft() {
  const record = await runTransaction("readonly", (store) => store.get(activeDraftKey));
  if (!record) {
    return null;
  }
  if (!Number.isFinite(record.savedAt) || Date.now() - record.savedAt > draftLifetimeMs) {
    await removeActiveDraft();
    return null;
  }
  return record.validation
    ? { validation: record.validation, dirty: Boolean(record.dirty) }
    : null;
}

export function removeActiveDraft() {
  return runTransaction("readwrite", (store) => store.delete(activeDraftKey));
}
