// Inside App Tabs
const tFiles = document.getElementById("tab-files");
const tShells = document.getElementById("tab-shells");
const fsDiv = document.getElementById("file-share");
const shDiv = document.getElementById("shell-handler");
tFiles.onclick = () => {
  tFiles.classList.add("active");
  tShells.classList.remove("active");
  fsDiv.classList.remove("hidden");
  shDiv.classList.add("hidden");
};
tShells.onclick = () => {
  tShells.classList.add("active");
  tFiles.classList.remove("active");
  shDiv.classList.remove("hidden");
  fsDiv.classList.add("hidden");
};

// Dummy Data
const files = ["secret.txt", "report.pdf", "data.csv", "payload.elf"];
const shells = [
  { id: 1, ip: "10.0.0.5", user: "root" },
  { id: 2, ip: "10.0.0.6", user: "admin" },
];
const fileList = document.getElementById("file-list");
const shellList = document.getElementById("shell-list");
files.forEach((name) => {
  const li = document.createElement("li");
  li.textContent = name;
  const dots = document.createElement("span");
  dots.textContent = "⋮";
  dots.classList.add("actions");
  li.appendChild(dots);
  fileList.appendChild(li);
});

shells.forEach((s) => {
  const li = document.createElement("li");
  li.innerHTML = `<span>${s.id} - ${s.ip}</span><span>${s.user}</span>`;
  shellList.appendChild(li);
});

// Context Menu Logic
const ctxMenu = document.getElementById("ctx-menu");
let currentTarget;
document.addEventListener("contextmenu", (e) => {
  const li = e.target.closest("#file-list li");
  if (li) {
    e.preventDefault();
    currentTarget = li;
    ctxMenu.style.top = e.pageY + "px";
    ctxMenu.style.left = e.pageX + "px";
    ctxMenu.style.display = "block";
  } else {
    ctxMenu.style.display = "none";
  }
});
document.addEventListener("click", (e) => {
  if (!e.target.closest(".context-menu")) ctxMenu.style.display = "none";
});
ctxMenu.querySelectorAll("li[data-action]").forEach((item) => {
  item.addEventListener("click", () => {
    const action = item.dataset.action;
    alert(`${action} on ${currentTarget.textContent}`);
    ctxMenu.style.display = "none";
  });
});
ctxMenu.querySelectorAll(".dropdown-menu li").forEach((item) => {
  item.addEventListener("click", () => {
    alert(
      `Exfiltrate via ${item.dataset.method} on ${currentTarget.textContent}`
    );
    ctxMenu.style.display = "none";
  });
});
