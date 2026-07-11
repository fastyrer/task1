// ===== Редактирование шаблона =====
'use strict';

templateInput.addEventListener("input", renderTemplateTags);

templateInput.addEventListener("keydown", (event) => {
  const selection = window.getSelection();
  if (!selection.rangeCount || !selection.isCollapsed) {
    return;
  }

  const range = selection.getRangeAt(0);
  if (event.key === "Backspace") {
    const targetTag = findTagAtCursor(range, "left");
    if (targetTag) {
      event.preventDefault();
      const offset = getCaretOffset(templateInput);
      const tagLength = targetTag.textContent.length;
      const text = templateInput.textContent || "";
      renderTemplateText(text.slice(0, offset - tagLength) + text.slice(offset));
      setCaretOffset(templateInput, offset - tagLength);
    }
  }

  if (event.key === "Delete") {
    const targetTag = findTagAtCursor(range, "right");
    if (targetTag) {
      event.preventDefault();
      const offset = getCaretOffset(templateInput);
      const tagLength = targetTag.textContent.length;
      const text = templateInput.textContent || "";
      renderTemplateText(text.slice(0, offset) + text.slice(offset + tagLength));
      setCaretOffset(templateInput, offset);
    }
  }
});

function findTagAtCursor(range, direction) {
  const container = range.startContainer;
  const offset = range.startOffset;
  const isText = container.nodeType === Node.TEXT_NODE;
  const length = (container.textContent || "").length;

  if (direction === "left") {
    if (isText && offset === 0) {
      const previous = container.previousSibling;
      if (previous?.classList?.contains("tag")) {
        return previous;
      }
    }
    if (isText && offset === length && container.parentNode?.classList?.contains("tag")) {
      return container.parentNode;
    }
  }

  if (direction === "right" && isText && offset === length) {
    const next = container.nextSibling;
    if (next?.classList?.contains("tag")) {
      return next;
    }
  }

  return null;
}

function insertAtCursor(editor, text) {
  const selection = window.getSelection();
  const offset = selection.rangeCount > 0 && editor.contains(selection.getRangeAt(0).startContainer)
    ? getCaretOffset(editor)
    : (editor.textContent || "").length;
  const currentText = editor.textContent || "";

  renderTemplateText(currentText.slice(0, offset) + text + currentText.slice(offset));
  setCaretOffset(editor, offset + text.length);
}

function getCaretOffset(element) {
  const selection = window.getSelection();
  if (!selection.rangeCount) {
    return 0;
  }

  const range = selection.getRangeAt(0);
  const preceding = document.createRange();
  preceding.selectNodeContents(element);
  preceding.setEnd(range.startContainer, range.startOffset);
  return preceding.toString().length;
}

function setCaretOffset(element, offset) {
  element.focus();
  const selection = window.getSelection();
  const range = document.createRange();
  const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT);
  let position = 0;

  while (walker.nextNode()) {
    const node = walker.currentNode;
    const length = node.textContent.length;
    if (position + length >= offset) {
      range.setStart(node, offset - position);
      range.collapse(true);
      selection.removeAllRanges();
      selection.addRange(range);
      return;
    }
    position += length;
  }

  range.selectNodeContents(element);
  range.collapse(false);
  selection.removeAllRanges();
  selection.addRange(range);
}

function renderTemplateTags() {
  const offset = getCaretOffset(templateInput);
  const text = templateInput.textContent || "";
  renderTemplateText(text);
  if (text) {
    setCaretOffset(templateInput, offset);
  }
}

function renderTemplateText(text) {
  if (!text) {
    templateInput.replaceChildren();
    return;
  }

  const fragment = document.createDocumentFragment();
  const tagPattern = /\{\{(.+?)\}\}/g;
  let position = 0;
  let match;

  while ((match = tagPattern.exec(text)) !== null) {
    fragment.appendChild(document.createTextNode(text.slice(position, match.index)));
    const tag = document.createElement("span");
    tag.className = "tag";
    tag.textContent = match[0];
    fragment.appendChild(tag);
    position = match.index + match[0].length;
  }
  fragment.appendChild(document.createTextNode(text.slice(position)));
  templateInput.replaceChildren(fragment);
}

function initConstructor() {
  constructorSection.style.display = "block";
  constructorHeaders.replaceChildren();

  currentHeaders.forEach((header) => {
    const item = document.createElement("li");
    item.textContent = `{{${header}}}`;
    item.style.cursor = "pointer";
    item.addEventListener("click", () => insertAtCursor(templateInput, `{{${header}}}`));
    constructorHeaders.appendChild(item);
  });

  placeholderHint.textContent = detectedPhoneColumn
    ? `Колонка телефона: ${detectedPhoneColumn}`
    : "Колонка телефона не определена.";
  notificationResults.style.display = "none";
  exportButton.style.display = "none";
  notificationError.textContent = "";
  templateInput.replaceChildren();
}
