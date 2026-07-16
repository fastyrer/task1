// ui.js - навигация приложения и общие функции отображения.

import { renderIcons } from "./icons.js";

const errorBlock = document.getElementById("errorBlock");

export function initWorkspaceUI() {
  renderIcons();
  initWorkspaceNavigation();
  initFilePanels();
  initHealthStatus();
}

function initWorkspaceNavigation() {
  document.querySelectorAll("[data-view-target]").forEach((control) => {
    control.addEventListener("click", () => showWorkspaceView(control.dataset.viewTarget));
  });
  document.querySelectorAll("[data-scroll-target]").forEach((control) => {
    control.addEventListener("click", () => {
      document.getElementById(control.dataset.scrollTarget)?.scrollIntoView({
        behavior: "smooth",
        block: "start",
      });
    });
  });
}

export function showWorkspaceView(viewName) {
  const target = document.querySelector(`.workspace-view[data-view="${viewName}"]`);
  if (!target) {
    return;
  }

  document.querySelectorAll(".workspace-view[data-view]").forEach((view) => {
    const active = view === target;
    view.hidden = !active;
    view.classList.toggle("is-active", active);
  });

  document.querySelectorAll(".nav-item[data-view-target]").forEach((item) => {
    const active = item.dataset.viewTarget === viewName;
    item.classList.toggle("is-active", active);
    if (active) {
      item.setAttribute("aria-current", "page");
    } else {
      item.removeAttribute("aria-current");
    }
  });

  window.scrollTo({ top: 0, behavior: "smooth" });
  document.dispatchEvent(new CustomEvent("workspace:view", { detail: { view: viewName } }));
}

function initFilePanels() {
  document.querySelectorAll(".workspace-tab[data-panel-target]").forEach((tab) => {
    tab.addEventListener("click", () => showFilePanel(tab.dataset.panelTarget));
  });
}

export function showFilePanel(panelName) {
  const target = document.querySelector(`.workspace-panel[data-panel="${panelName}"]`);
  if (!target) {
    return;
  }

  document.querySelectorAll(".workspace-panel[data-panel]").forEach((panel) => {
    const active = panel === target;
    panel.hidden = !active;
    panel.classList.toggle("is-active", active);
  });

  document.querySelectorAll(".workspace-tab[data-panel-target]").forEach((tab) => {
    const active = tab.dataset.panelTarget === panelName;
    tab.classList.toggle("is-active", active);
    tab.setAttribute("aria-selected", String(active));
  });
}

function initHealthStatus() {
  const status = document.getElementById("dbStatus");
  const statusText = document.getElementById("dbStatusText");
  if (!status || !statusText) {
    return;
  }

  const check = async () => {
    status.dataset.state = "checking";
    statusText.textContent = "Проверяем PostgreSQL";
    try {
      const response = await fetch("/api/health", { headers: { Accept: "application/json" } });
      const payload = await response.json();
      if (!response.ok || payload.status !== "ok") {
        throw new Error("storage unavailable");
      }
      status.dataset.state = "ok";
      statusText.textContent = "PostgreSQL подключена";
    } catch (error) {
      status.dataset.state = "error";
      statusText.textContent = "PostgreSQL недоступна";
    }
  };

  check();
  window.setInterval(check, 30000);
}

export function showError(message) {
  errorBlock.textContent = message;
}

export function clearError() {
  errorBlock.textContent = "";
}

export function formatWarning(warning) {
  const parts = [];
  if (warning.row) {
    parts.push(`строка ${warning.row}`);
  }
  if (warning.column) {
    parts.push(`колонка "${warning.column}"`);
  }
  parts.push(warning.message || "Предупреждение");
  return parts.join(": ");
}

export function formatBytes(value) {
  if (value < 1024) {
    return `${value} Б`;
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} КБ`;
  }

  return `${(value / 1024 / 1024).toFixed(1)} МБ`;
}
