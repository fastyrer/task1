// app.js - точка инициализации независимых frontend-модулей.

import { initContacts } from "./contacts.js";
import { initNotifications } from "./notifications.js";
import { initSearch } from "./search.js";
import { initTemplateEditor } from "./template-editor.js";
import { initCollapsiblePanels } from "./ui.js";
import { initUpload } from "./upload.js";

initCollapsiblePanels();
initTemplateEditor();
initNotifications();

const searchController = initSearch();
const contactsController = initContacts();
initUpload({ searchController, contactsController });
