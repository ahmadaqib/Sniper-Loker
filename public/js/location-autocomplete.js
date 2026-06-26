(function () {
  const input = document.getElementById("location");
  const datalist = document.getElementById("locationOptions");
  if (!input || !datalist) return;

  const cacheKey = "loker_radar_locations_v1";
  const cacheTTL = 7 * 24 * 60 * 60 * 1000;
  const apiBase = "https://www.emsifa.com/api-wilayah-indonesia/api";
  const fallback = [
    "Jakarta",
    "Bandung",
    "Surabaya",
    "Medan",
    "Makassar",
    "Semarang",
    "Yogyakarta",
    "Denpasar",
    "Balikpapan",
    "Samarinda",
    "Manado",
    "Palembang",
    "Batam",
    "Pekanbaru",
    "Malang",
  ].map((name) => ({ value: name, label: "Kota populer" }));

  let locations = [];
  let loading = false;

  hydrateFromCache();
  renderOptions(input.value);

  input.addEventListener("focus", () => loadLocations());
  input.addEventListener("input", debounce(() => {
    renderOptions(input.value);
    if (!locations.length) loadLocations();
  }, 120));

  async function loadLocations() {
    if (loading || locations.length) return;
    loading = true;
    try {
      const provinces = await fetchJSON(`${apiBase}/provinces.json`);
      const regencyGroups = [];
      for (const province of provinces) {
        regencyGroups.push(fetchJSON(`${apiBase}/regencies/${province.id}.json`).then((items) => ({ province, items })));
      }
      const settled = await Promise.allSettled(regencyGroups);
      locations = settled
        .filter((result) => result.status === "fulfilled")
        .flatMap((result) => result.value.items.map((item) => ({
          value: cleanRegionName(item.name),
          label: titleCase(result.value.province.name),
          raw: item.name,
        })))
        .filter((item, index, arr) => item.value && arr.findIndex((other) => other.value === item.value) === index)
        .sort((a, b) => a.value.localeCompare(b.value));
      saveCache(locations);
      renderOptions(input.value);
    } catch (error) {
      locations = fallback;
      renderOptions(input.value);
    } finally {
      loading = false;
    }
  }

  async function fetchJSON(url) {
    const response = await fetch(url, { cache: "force-cache" });
    if (!response.ok) throw new Error(`location api ${response.status}`);
    return response.json();
  }

  function renderOptions(query) {
    const needle = normalize(query);
    const pool = locations.length ? locations : fallback;
    const matches = pool
      .filter((item) => !needle || normalize(item.value).includes(needle) || normalize(item.raw || "").includes(needle))
      .slice(0, 30);

    datalist.replaceChildren();
    matches.forEach((item) => {
      const option = document.createElement("option");
      option.value = item.value;
      option.label = item.label;
      datalist.appendChild(option);
    });
  }

  function hydrateFromCache() {
    try {
      const cached = JSON.parse(localStorage.getItem(cacheKey) || "null");
      if (!cached || Date.now() - cached.savedAt > cacheTTL || !Array.isArray(cached.locations)) {
        locations = fallback;
        return;
      }
      locations = cached.locations;
    } catch (_) {
      locations = fallback;
    }
  }

  function saveCache(items) {
    try {
      localStorage.setItem(cacheKey, JSON.stringify({ savedAt: Date.now(), locations: items }));
    } catch (_) {
      // Browser storage may be disabled; autocomplete still works for this session.
    }
  }

  function cleanRegionName(value) {
    return titleCase(value.replace(/^KOTA\s+/i, "").replace(/^KABUPATEN\s+/i, ""));
  }

  function titleCase(value) {
    return value.toLowerCase().replace(/\b\w/g, (letter) => letter.toUpperCase());
  }

  function normalize(value) {
    return String(value || "").toLowerCase().replace(/[^a-z0-9]+/g, " ").trim();
  }

  function debounce(fn, wait) {
    let timer;
    return (...args) => {
      clearTimeout(timer);
      timer = setTimeout(() => fn(...args), wait);
    };
  }
})();

