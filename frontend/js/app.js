// app.js - загружает представления и инициализирует независимые frontend-модули.

const viewPaths = [
  "/views/files.html",
  "/views/contacts.html",
  "/views/notifications.html",
];

async function loadWorkspaceViews() {
  const mount = document.getElementById("workspaceMount");
  const responses = await Promise.all(viewPaths.map((path) => fetch(path)));
  const failed = responses.find((response) => !response.ok);
  if (failed) {
    throw new Error(`load workspace view: ${failed.status}`);
  }

  const markup = await Promise.all(responses.map((response) => response.text()));
  const fragment = document.createDocumentFragment();
  markup.forEach((html) => {
    const template = document.createElement("template");
    template.innerHTML = html.trim();
    fragment.appendChild(template.content);
  });
  mount.replaceChildren(fragment);
}

async function bootstrap() {
  await loadWorkspaceViews();

  const [contacts, notifications, search, templateEditor, ui, upload] = await Promise.all([
    import("./contacts.js"),
    import("./notifications.js"),
    import("./search.js"),
    import("./template-editor.js"),
    import("./ui.js"),
    import("./upload.js"),
  ]);

  ui.initWorkspaceUI();
  templateEditor.initTemplateEditor();
  notifications.initNotifications();

  const searchController = search.initSearch();
  const contactsController = contacts.initContacts();
  upload.initUpload({ searchController, contactsController });
}

bootstrap().catch(() => {
  const mount = document.getElementById("workspaceMount");
  const message = document.createElement("div");
  message.className = "error";
  message.textContent = "Не удалось загрузить интерфейс приложения.";
  mount.replaceChildren(message);
});
