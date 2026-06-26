(function () {
  const state = {
    jobs: new Map(),
    client: null,
    newCount: 0,
    batchCount: 0,
    keyword: "",
    location: "",
  };

  const form = document.getElementById("searchForm");
  const keyword = document.getElementById("keyword");
  const locationInput = document.getElementById("location");
  const filterToggle = document.getElementById("filterToggle");
  const advancedFilters = document.getElementById("advancedFilters");
  const jobList = document.getElementById("jobList");
  const skeletonList = document.getElementById("skeletonList");
  const emptyState = document.getElementById("emptyState");
  const emptyCopy = document.getElementById("emptyCopy");
  const errorState = document.getElementById("errorState");
  const statusDot = document.getElementById("statusDot");
  const statusText = document.getElementById("statusText");
  const jobCount = document.getElementById("jobCount");
  const updatedAt = document.getElementById("updatedAt");
  const newBadge = document.getElementById("newBadge");
  const batchCount = document.getElementById("batchCount");
  const resultCaption = document.getElementById("resultCaption");
  const sortSelect = document.getElementById("sortSelect");
  const template = document.getElementById("jobCardTemplate");

  filterToggle.addEventListener("click", () => {
    const open = advancedFilters.hidden;
    advancedFilters.hidden = !open;
    filterToggle.setAttribute("aria-expanded", String(open));
  });

  form.addEventListener("submit", (event) => {
    event.preventDefault();
    startSearch();
  });

  document.getElementById("refreshButton").addEventListener("click", () => startSearch());
  document.getElementById("retryButton").addEventListener("click", () => startSearch());

  document.querySelectorAll("[data-keyword]").forEach((button) => {
    button.addEventListener("click", () => {
      keyword.value = button.dataset.keyword;
      startSearch();
    });
  });

  sortSelect.addEventListener("change", renderJobs);

  function startSearch() {
    state.keyword = keyword.value.trim();
    state.location = locationInput.value.trim();
    state.jobs.clear();
    state.newCount = 0;
    state.batchCount = 0;
    setLoading(true);
    setError(false);
    updateCounts();

    if (state.client) state.client.close();
    state.client = new JobsRealtimeClient({
      keyword: state.keyword,
      location: state.location,
      onJobs: (jobs) => addJobs(jobs, true),
      onStatus: setStatus,
    });
    state.client.connect();
    triggerScrape();
    pollInitial();

    resultCaption.textContent = `Memantau lowongan "${state.keyword || "semua"}" di ${state.location || "semua lokasi"}.`;
    emptyCopy.textContent = `Belum ada lowongan "${state.keyword || "terbaru"}" di ${state.location || "lokasi pilihan"}. Sistem akan terus mencari secara otomatis setiap 5 menit.`;
  }

  async function triggerScrape() {
    try {
      const response = await fetch("/api/search", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ keyword: state.keyword, location: state.location }),
      });
      if (!response.ok) {
        const payload = await response.json().catch(() => ({}));
        throw new Error(payload.detail || payload.error || "Gagal memulai pencarian.");
      }
    } catch (error) {
      setError(true, error.message);
    }
  }

  async function pollInitial() {
    try {
      const params = new URLSearchParams({ keyword: state.keyword, location: state.location, limit: "50" });
      const response = await fetch(`/api/jobs?${params.toString()}`);
      if (!response.ok) {
        let detail = "Database belum siap. Pastikan MongoDB berjalan.";
        try {
          const payload = await response.json();
          detail = payload.detail || payload.error || detail;
        } catch (_) {
          detail = response.statusText || detail;
        }
        throw new Error(detail);
      }
      const payload = await response.json();
      addJobs(payload.jobs || [], false);
    } catch (error) {
      setError(true, error.message);
    } finally {
      setLoading(false);
    }
  }

  function addJobs(jobs, markNew) {
    if (!Array.isArray(jobs)) return;
    let added = 0;
    jobs.forEach((job) => {
      const id = job.id || job.content_hash || `${job.source}:${job.external_id || job.source_url || job.title}`;
      if (!state.jobs.has(id)) added++;
      state.jobs.set(id, { ...job, __new: markNew });
    });
    if (markNew && added > 0) {
      state.newCount += added;
      state.batchCount += 1;
    }
    renderJobs();
    updateCounts();
  }

  function renderJobs() {
    const jobs = Array.from(state.jobs.values());
    if (sortSelect.value === "company") {
      jobs.sort((a, b) => String(a.company || "").localeCompare(String(b.company || "")));
    } else {
      jobs.sort((a, b) => new Date(b.last_seen_at || b.posted_at || 0) - new Date(a.last_seen_at || a.posted_at || 0));
    }

    jobList.replaceChildren();
    jobs.forEach((job) => jobList.appendChild(renderJob(job)));
    emptyState.hidden = jobs.length > 0 || !skeletonList.hidden;
  }

  function renderJob(job) {
    const node = template.content.firstElementChild.cloneNode(true);
    if (job.__new) node.classList.add("is-new");
    node.querySelector(".company-logo").textContent = initials(job.company || job.source || "LR");
    node.querySelector(".job-title").textContent = job.title || "Lowongan tanpa judul";
    node.querySelector(".job-company").textContent = job.company || "Perusahaan tidak dicantumkan";
    node.querySelector(".job-location").textContent = job.location || "Lokasi tidak dicantumkan";
    node.querySelector(".source-badge").textContent = `via ${sourceName(job.source)}`;
    node.querySelector(".job-time").textContent = relativeTime(job.last_seen_at || job.posted_at || job.first_seen_at);
    node.querySelector(".job-description").textContent = stripHTML(job.description) || "Deskripsi belum tersedia dari sumber.";

    const skills = inferSkills(job);
    const skillsNode = node.querySelector(".skills");
    skills.slice(0, 7).forEach((skill) => {
      const tag = document.createElement("span");
      tag.textContent = skill;
      skillsNode.appendChild(tag);
    });
    if (skills.length > 7) {
      const more = document.createElement("span");
      more.textContent = `+${skills.length - 7} lagi`;
      skillsNode.appendChild(more);
    }

    const apply = node.querySelector(".apply-button");
    if (job.apply_url || job.source_url) {
      apply.href = job.apply_url || job.source_url;
    } else {
      apply.hidden = true;
    }

    const desc = node.querySelector(".job-description");
    node.querySelector(".details-button").addEventListener("click", (event) => {
      const open = desc.classList.toggle("is-open");
      event.currentTarget.textContent = open ? "Ringkas" : "Selengkapnya";
    });

    node.querySelector(".save-button").addEventListener("click", (event) => {
      event.currentTarget.classList.toggle("is-saved");
    });

    return node;
  }

  function setStatus(status) {
    statusDot.className = "status-dot";
    if (status === "connected") {
      statusDot.classList.add("status-dot--live");
      statusText.textContent = "Live";
    } else if (status === "reconnecting") {
      statusDot.classList.add("status-dot--warn");
      statusText.textContent = "Menghubungkan ulang";
    } else {
      statusDot.classList.add("status-dot--dead");
      statusText.textContent = "Polling aktif";
    }
  }

  function setLoading(loading) {
    skeletonList.hidden = !loading;
    emptyState.hidden = loading || state.jobs.size > 0;
  }

  function setError(show, message = "") {
    errorState.hidden = !show;
    if (show && message) {
      errorState.querySelector("span").textContent = message;
    }
  }

  function updateCounts() {
    jobCount.textContent = String(state.jobs.size);
    batchCount.textContent = String(state.batchCount);
    updatedAt.textContent = state.jobs.size ? `Diperbarui ${relativeTime(new Date().toISOString())}` : "Belum diperbarui";
    newBadge.hidden = state.newCount === 0;
    newBadge.textContent = `${state.newCount} baru`;
  }

  function inferSkills(job) {
    const text = `${job.title || ""} ${job.description || ""}`.toLowerCase();
    const dictionary = ["Go", "PHP", "Laravel", "MongoDB", "SQL", "Excel", "Pajak", "SAP", "React", "Vue", "Data", "API", "Sales", "WFH"];
    const found = dictionary.filter((skill) => text.includes(skill.toLowerCase()));
    return found.length ? found : ["Lowongan", "Terbaru"];
  }

  function sourceName(source) {
    return {
      loker_id: "Loker.id",
      karir_com: "Karir.com",
      indeed: "Indeed",
      glints: "Glints",
    }[source] || "Sumber";
  }

  function initials(value) {
    return value.split(/\s+/).filter(Boolean).slice(0, 2).map((word) => word[0]).join("").toUpperCase();
  }

  function stripHTML(value) {
    const div = document.createElement("div");
    div.innerHTML = value || "";
    return div.textContent || div.innerText || "";
  }

  function relativeTime(value) {
    if (!value) return "Baru saja";
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "Baru saja";
    const diff = Math.max(0, Date.now() - date.getTime());
    const minutes = Math.floor(diff / 60000);
    if (minutes < 1) return "Baru saja";
    if (minutes < 60) return `${minutes} mnt lalu`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours} jam lalu`;
    return `${Math.floor(hours / 24)} hari lalu`;
  }

  setLoading(false);
  updateCounts();
})();
