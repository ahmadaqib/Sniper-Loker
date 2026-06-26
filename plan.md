# Loker Radar — Comprehensive Implementation Plan

**Domain:** loker.elips.site | **Stack:** Goravel (Go) + MongoDB + WebSockets
**Pendekatan Scraping:** Pure HTML scraping + JSON endpoint publik — tanpa API key, tanpa langganan berbayar

---

## 1. Gap Analysis — Apa yang Hilang dari PRD Awal

### 1.1 Gap Teknis

| Area                                   | Gap                                                                         | Implikasi                                                                                                      |
| -------------------------------------- | --------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| **Auth & Session**                     | PRD tidak menyebut autentikasi pengguna sama sekali                         | Siapa saja bisa query semua data; abuse sangat mudah                                                           |
| **Rate Limiting (Inbound)**            | Hanya outbound rate limiter yang disebut                                    | Tidak ada perlindungan dari pengguna yang spam-query ke server sendiri                                         |
| **Scraper Mode**                       | PRD menyebut "API publik" — ambigu antara API resmi vs endpoint JSON publik | Harus ditegaskan: kita scraping HTML + JSON publik, zero API key                                               |
| **Queue Backend**                      | PRD menyebut "Queue Worker" tapi tidak tentukan driver                      | Redis? In-memory? Goravel butuh driver eksplisit                                                               |
| **Scraper Contract / Interface**       | Disebutkan "fleksibel" tapi tidak ada interface definition                  | Developer berikutnya tidak tahu cara menambah sumber baru                                                      |
| **Anti-Detection Strategy**            | Hanya disebut "delay 1-2 detik" — jauh dari cukup                           | Tanpa User-Agent rotation, header realistis, dan TLS fingerprint yang benar, akan terblokir dalam hitungan jam |
| **HTML Parser Strategy**               | Tidak disebut library parsing sama sekali                                   | Gunakan `goquery` (Go port dari jQuery) atau `colly`                                                           |
| **JavaScript-Rendered Pages**          | Beberapa job board (Glints, Kalibrr) render via JS                          | Pure HTTP GET tidak akan mendapatkan konten — butuh strategi khusus                                            |
| **Error Taxonomy**                     | Disebutkan "catat log bila sumber gagal"                                    | Tidak ada klasifikasi: 4xx vs 5xx vs timeout vs parse error — beda penanganannya                               |
| **Data Versioning / Schema Migration** | MongoDB schemaless, tapi struktur HTML bisa berubah                         | Tidak ada strategi saat selector CSS sumber berubah                                                            |
| **Search Subscription Lifecycle**      | `SearchQueries` disimpan, tapi kapan dihapus?                               | Query usang akan terus dijadwalkan secara sia-sia                                                              |
| **Deduplication Strategy Detail**      | "Cek judul+perusahaan+lokasi"                                               | Bagaimana jika satu perusahaan post ulang dengan judul sedikit berbeda? Butuh content hash                     |
| **WebSocket Reconnection**             | PRD tidak bahas client-side reconnect                                       | Jika koneksi putus, user tidak akan tahu ada data baru                                                         |
| **Environment Parity**                 | Setup hanya untuk macOS (Homebrew)                                          | Tidak ada panduan untuk Linux/Docker untuk dev dan prod                                                        |
| **Pagination / Infinite Scroll**       | Tidak disebut                                                               | Jika ada 1000 hasil, semua dikembalikan sekaligus?                                                             |
| **Testing Strategy**                   | Tidak ada sama sekali                                                       | Unit test scraper? Integration test WebSocket?                                                                 |

### 1.2 Gap UX/UI

| Area                             | Gap                                                                   |
| -------------------------------- | --------------------------------------------------------------------- |
| **Empty State**                  | Apa yang ditampilkan saat hasil 0?                                    |
| **Loading Skeleton**             | Spinner generik disebut, tapi tidak ada strategi skeleton screen      |
| **Offline/Connection Lost**      | Tidak ada UI untuk kondisi WebSocket disconnect                       |
| **Onboarding / First-time User** | Tidak ada. Pengguna baru tidak tahu harus search apa                  |
| **Aksesibilitas (a11y)**         | Disebut "pertimbangkan" tapi tidak ada spesifikasi (WCAG level?)      |
| **Error State Cards**            | Tidak ada desain untuk kartu "gagal memuat"                           |
| **Mobile Navigation**            | Form + daftar hasil di layar kecil tidak dibahas                      |
| **Color Scheme**                 | Tidak ada. "Warna aksen kontras" tapi tidak ada hex, tidak ada sistem |
| **Typography Scale**             | Tidak ada. Font tidak ditentukan                                      |
| **Micro-interactions**           | Disebutkan "animasi halus" tapi tidak ada spesifikasi                 |

---

## 2. Strategi Scraping — Pure HTML & JSON Publik

### 2.1 Filosofi: Zero Dependency pada API Berbayar

Semua data diambil dari endpoint yang dapat diakses browser biasa tanpa login atau API key. Ada dua jenis sumber:

**Tipe A — JSON Endpoint Publik (tidak butuh render JS)**
Beberapa platform secara internal menggunakan endpoint JSON yang bisa diakses langsung jika kita tahu URL-nya dan membawa header yang tepat. Contoh: endpoint pencarian JobStreet menghasilkan JSON jika request dikirim dengan `Accept: application/json`.

**Tipe B — HTML Scraping dengan goquery**
Platform yang merender konten server-side (SSR). HTML response langsung bisa di-parse dengan `goquery`.

**Tipe C — HTML dengan JSON embedded (JSON-LD / Next.js `__NEXT_DATA__`)**
Banyak platform modern (Glints, Kalibrr) menggunakan Next.js dan menyematkan seluruh data halaman sebagai JSON di dalam tag `<script id="__NEXT_DATA__">`. Ini jauh lebih stabil daripada parsing CSS selector karena JSON structured.

**Yang DIHINDARI:**

- Headless browser (Playwright/Puppeteer) untuk production — terlalu berat, IP mudah terdeteksi
- API berbayar (Apify, ScrapingBee, dll)
- Scraping yang butuh login/session

### 2.2 Target Sumber Data (Gratis, Publik)

| Sumber                | Tipe                     | Endpoint Pattern                                                                    | Catatan                                                                                   |
| --------------------- | ------------------------ | ----------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| **Loker.id**          | Tipe B (HTML SSR)        | `/lowongan?q={keyword}&l={location}`                                                | SSR penuh, sangat mudah di-parse                                                          |
| **Karir.com**         | Tipe B (HTML SSR)        | `/job-search?keywords={keyword}&location={location}`                                | SSR, selector stabil                                                                      |
| **JobStreet ID**      | Tipe A (JSON publik)     | `/api/chalice-search/v4/search?...`                                                 | Endpoint internal JSON, butuh header `X-Sol-Pub-Api` palsu tapi tidak butuh API key nyata |
| **Indeed ID**         | Tipe C (JSON embedded)   | `/jobs?q={keyword}&l={location}`                                                    | Data ada di `window.mosaic.providerData` dalam script tag                                 |
| **Kalibrr**           | Tipe C (`__NEXT_DATA__`) | `/job-board?search={keyword}`                                                       | Next.js app, data di `__NEXT_DATA__`                                                      |
| **LinkedIn (public)** | Tipe A (JSON publik)     | `/jobs-guest/jobs/api/seeMoreJobPostings/?keywords={k}&location={l}&start={offset}` | Endpoint JSON tanpa auth untuk listing publik                                             |
| **Glints ID**         | Tipe C (`__NEXT_DATA__`) | `/en-id/jobs?keyword={k}&country=ID&locationName={l}`                               | Next.js, data di `__NEXT_DATA__`                                                          |

> **Catatan Penting:** Struktur HTML dan endpoint internal sumber-sumber ini bisa berubah kapan saja. Ini adalah resiko inheren dari scraping. Strategi mitigasinya ada di Section 4 (Error Taxonomy) dan Section 6 (Failure Scenarios).

### 2.3 Library yang Digunakan

```
go get github.com/gocolly/colly/v2        // HTTP client dengan built-in anti-detection
go get github.com/PuerkitoBio/goquery     // HTML parsing (jQuery-like)
go get golang.org/x/net/html              // HTML tokenizer level rendah (fallback)
```

**Mengapa Colly, bukan net/http biasa?**

- Built-in rate limiting per domain
- Cookie jar otomatis (session persistence)
- Concurrent scraping dengan goroutine pool
- Built-in retry logic
- Request caching (hindari re-fetch halaman yang sama)

### 2.4 Teknik Ekstraksi per Tipe

**Tipe A — JSON Endpoint:**

```go
resp, _ := client.Get(url) // dengan header palsu
var result GlintsAPIResponse
json.NewDecoder(resp.Body).Decode(&result)
```

**Tipe B — HTML goquery:**

```go
doc, _ := goquery.NewDocumentFromReader(resp.Body)
doc.Find(".job-card").Each(func(i int, s *goquery.Selection) {
    title := s.Find(".job-title").Text()
    company := s.Find(".company-name").Text()
    // dst.
})
```

**Tipe C — JSON embedded di script tag:**

```go
doc.Find("script#__NEXT_DATA__").Each(func(i int, s *goquery.Selection) {
    var nextData NextJSData
    json.Unmarshal([]byte(s.Text()), &nextData)
    jobs := nextData.Props.PageProps.Jobs
})
```

---

## 3. Anti-Detection Strategy — Komprehensif

Ini bagian paling kritis. "Delay 1-2 detik" saja tidak cukup. Detection modern bekerja di banyak lapisan.

### 3.1 Layer 1 — TLS Fingerprint (JA3)

Masalah: Go's `net/http` menghasilkan TLS ClientHello yang berbeda dari browser nyata. Server canggih (Cloudflare, Akamai) mendeteksi ini sebelum melihat header HTTP.

Solusi: Gunakan `utls` (uTLS) yang memungkinkan kita meniru TLS fingerprint Chrome/Firefox:

```go
// go get github.com/refraction-networking/utls
import utls "github.com/refraction-networking/utls"

tlsConfig := &utls.Config{InsecureSkipVerify: false}
conn, _ := utls.Dial("tcp", host+":443", tlsConfig)
// Gunakan HelloChrome_Auto untuk fingerprint Chrome terbaru
conn.ApplyPreset(utls.HelloChrome_Auto)
```

Ini membuat handshake TLS kita identik dengan Chrome. Efektif melawan deteksi berbasis JA3/JA3S.

### 3.2 Layer 2 — HTTP/2 Fingerprint

Chrome menggunakan HTTP/2 dengan urutan frame dan setting tertentu. `golang.org/x/net/http2` mengirim setting default yang berbeda.

Solusi: Gunakan `cycletls` atau set header HTTP/2 secara manual:

```go
// go get github.com/Danny-Dasilva/CycleTLS/cycletls
// Library ini menggabungkan uTLS + HTTP/2 fingerprint Chrome
client := cycletls.Init()
response, _ := client.Do(url, cycletls.Options{
    Ja3:       "771,4865-4866-4867-49195-...",  // JA3 Chrome 120
    UserAgent: userAgent,
}, "GET")
```

### 3.3 Layer 3 — HTTP Header Realism

Header harus dikirim dalam urutan yang sama dengan Chrome, dengan nilai yang realistis:

```go
// URUTAN HEADER PENTING — browser mengirim dalam urutan ini
var realisticHeaders = []struct{ key, val string }{
    {"User-Agent",                currentUA},
    {"Accept",                    "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8"},
    {"Accept-Language",           "id-ID,id;q=0.9,en-US;q=0.8,en;q=0.7"},
    {"Accept-Encoding",           "gzip, deflate, br"},
    {"DNT",                       "1"},
    {"Connection",                "keep-alive"},
    {"Upgrade-Insecure-Requests", "1"},
    {"Sec-Fetch-Dest",            "document"},
    {"Sec-Fetch-Mode",            "navigate"},
    {"Sec-Fetch-Site",            "none"},
    {"Sec-Fetch-User",            "?1"},
    {"Cache-Control",             "max-age=0"},
}
```

**Yang tidak boleh dilupakan:**

- `Sec-CH-UA`: Client Hints header. Chrome selalu mengirim ini
- `Sec-CH-UA-Mobile`: `?0` untuk desktop
- `Sec-CH-UA-Platform`: `"Windows"` atau `"macOS"`

### 3.4 Layer 4 — User-Agent Rotation

Jangan gunakan UA yang sama untuk semua request. Buat pool dari UA Chrome terbaru yang nyata:

```go
var userAgentPool = []string{
    // Chrome 120 Windows
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
    // Chrome 120 macOS
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
    // Chrome 119 Windows
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
    // Edge 120
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
}

func getRandomUA() string {
    return userAgentPool[rand.Intn(len(userAgentPool))]
}
```

**Aturan rotasi:**

- Satu sesi scraping (satu run scheduler) = satu UA yang sama per domain
- Antar sesi = ganti UA
- Jangan ganti UA di tengah sesi — justru mencurigakan

### 3.5 Layer 5 — Request Timing & Jitter

Delay yang konstan (selalu tepat 2 detik) sangat mudah dideteksi sebagai bot. Manusia tidak klik dengan ritme sempurna.

```go
func humanDelay(base, jitter time.Duration) {
    // Tambahkan jitter random: ±jitter dari base
    delay := base + time.Duration(rand.Int63n(int64(jitter*2))) - jitter
    time.Sleep(delay)
}

// Penggunaan:
humanDelay(2*time.Second, 800*time.Millisecond)
// Hasil: delay antara 1.2 - 2.8 detik, distribusi merata
```

**Strategi timing per sumber:**

| Sumber    | Base Delay | Jitter | Max per jam |
| --------- | ---------- | ------ | ----------- |
| Loker.id  | 3s         | ±1s    | 30 req      |
| Karir.com | 3s         | ±1.5s  | 25 req      |
| JobStreet | 5s         | ±2s    | 15 req      |
| Indeed    | 4s         | ±1.5s  | 20 req      |
| LinkedIn  | 6s         | ±3s    | 10 req      |
| Glints    | 4s         | ±2s    | 18 req      |
| Kalibrr   | 3s         | ±1s    | 28 req      |

### 3.6 Layer 6 — Referer & Navigation Context

Browser nyata selalu punya Referer yang masuk akal. Request yang datang "dari mana-mana" tanpa Referer mencurigakan.

```go
// Simulasikan navigasi organik:
// 1. Buka homepage dulu
// 2. Baru ke halaman pencarian

func (s *GlintsSource) Fetch(keyword, location string) {
    // Step 1: "kunjungi" homepage (tidak perlu parse, cukup GET)
    s.client.Get("https://glints.com/id", withReferer(""))
    humanDelay(1500*time.Millisecond, 500*time.Millisecond)

    // Step 2: request pencarian dengan referer homepage
    s.client.Get(searchURL, withReferer("https://glints.com/id"))
}
```

### 3.7 Layer 7 — Cookie & Session Persistence

Bot yang tidak menyimpan cookie terlihat jelas — browser nyata selalu punya cookie session.

```go
// Colly otomatis handle cookie jar
c := colly.NewCollector(
    colly.AllowURLRevisit(), // untuk pagination
)

// Simpan cookies antar request dalam satu sesi
cookieJar, _ := cookiejar.New(nil)
c.SetCookieJar(cookieJar)
```

### 3.8 Layer 8 — IP Cooldown & Circuit Breaker

Jika terdeteksi (dapat 429/403), jangan langsung retry. Implementasikan circuit breaker:

```go
type CircuitBreaker struct {
    failures    int
    maxFailures int
    cooldown    time.Duration
    resetAt     time.Time
    state       string // "closed" | "open" | "half-open"
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    if cb.state == "open" {
        if time.Now().Before(cb.resetAt) {
            return ErrCircuitOpen // tolak request, jangan kirim
        }
        cb.state = "half-open"
    }

    err := fn()
    if err != nil {
        cb.failures++
        if cb.failures >= cb.maxFailures {
            cb.state = "open"
            cb.resetAt = time.Now().Add(cb.cooldown)
        }
        return err
    }

    cb.failures = 0
    cb.state = "closed"
    return nil
}
```

**Circuit breaker config per sumber:**

- Threshold: 3 kegagalan berturut-turut
- Cooldown open: 30 menit
- Half-open: coba 1 request, jika berhasil → closed

### 3.9 Layer 9 — Response Validation

Jangan asumsikan response valid hanya karena HTTP 200. Cloudflare dan sistem challenge sering mengembalikan 200 dengan halaman "Checking your browser...".

```go
func isValidJobPage(body string) bool {
    // Deteksi halaman challenge / CAPTCHA
    if strings.Contains(body, "cf-challenge") { return false }
    if strings.Contains(body, "Ray ID") { return false }
    if strings.Contains(body, "Just a moment") { return false }
    if strings.Contains(body, "Enable JavaScript") { return false }
    if len(body) < 1000 { return false } // halaman terlalu kecil = redirect/block
    return true
}
```

### 3.10 Matriks Anti-Detection — Apa yang Melawan Siapa

| Teknik                 | Bot Detector Sederhana | Cloudflare Basic | Cloudflare Advanced | DataDome         | PerimeterX       |
| ---------------------- | ---------------------- | ---------------- | ------------------- | ---------------- | ---------------- |
| User-Agent realistis   | ✅ bypass              | ✅ bypass        | Partial             | Partial          | ❌               |
| Header realistis       | ✅ bypass              | ✅ bypass        | Partial             | Partial          | ❌               |
| TLS fingerprint (uTLS) | ✅ bypass              | ✅ bypass        | ✅ bypass           | Partial          | Partial          |
| HTTP/2 fingerprint     | ✅ bypass              | ✅ bypass        | ✅ bypass           | ✅ bypass        | Partial          |
| Request timing jitter  | ✅ bypass              | ✅ bypass        | ✅ bypass           | Partial          | Partial          |
| Cookie persistence     | ✅ bypass              | ✅ bypass        | ✅ bypass           | ✅ bypass        | Partial          |
| Referer chain          | ✅ bypass              | ✅ bypass        | ✅ bypass           | Partial          | Partial          |
| Circuit breaker        | N/A                    | ✅ melindungi IP | ✅ melindungi IP    | ✅ melindungi IP | ✅ melindungi IP |

> **Target realistis kita:** Loker.id, Karir.com, Indeed, Kalibrr menggunakan deteksi sederhana-medium. Kombinasi Layer 1-7 sudah cukup. LinkedIn dan Glints lebih agresif — Layers 1-8 diperlukan.

---

## 4. Error Taxonomy — Klasifikasi Lengkap

Bukan hanya "catat log". Setiap error class punya penanganan berbeda.

```
ERROR CLASS
├── NETWORK
│   ├── E-NET-01: Connection timeout (> threshold)
│   ├── E-NET-02: DNS resolution failure
│   └── E-NET-03: SSL/TLS handshake failure
│
├── HTTP
│   ├── E-HTTP-429: Rate limited (Too Many Requests)
│   ├── E-HTTP-403: Forbidden (IP blocked / auth required)
│   ├── E-HTTP-404: URL struktur sumber berubah
│   ├── E-HTTP-503: Sumber sedang maintenance
│   └── E-HTTP-5xx: Sumber error internal
│
├── PARSE
│   ├── E-PARSE-01: Selector CSS tidak ditemukan (struktur HTML berubah)
│   ├── E-PARSE-02: JSON decode gagal (format berubah)
│   ├── E-PARSE-03: Field wajib kosong (title/company null)
│   └── E-PARSE-04: Response challenge/CAPTCHA terdeteksi
│
├── DATA
│   ├── E-DATA-01: Duplikat terdeteksi (skip, bukan error)
│   ├── E-DATA-02: Data tidak valid (URL apply tidak valid, dll)
│   └── E-DATA-03: Encoding masalah (karakter aneh)
│
└── SYSTEM
    ├── E-SYS-01: MongoDB write failure
    ├── E-SYS-02: Queue worker crash
    └── E-SYS-03: Scheduler tidak berjalan
```

**Action per error class:**

| Error      | Action                                  | Alert?                | Cooldown?          |
| ---------- | --------------------------------------- | --------------------- | ------------------ |
| E-NET-01   | Retry 3x exponential backoff            | Setelah 3x gagal      | Tidak              |
| E-NET-02   | Log + skip sumber ini                   | Ya                    | 1 jam              |
| E-HTTP-429 | Circuit breaker OPEN                    | Ya                    | 30 menit           |
| E-HTTP-403 | Circuit breaker OPEN + ganti UA         | Ya                    | 1 jam              |
| E-HTTP-404 | Log CRITICAL (struktur berubah)         | Ya (butuh fix manual) | Nonaktifkan sumber |
| E-HTTP-503 | Retry setelah 10 menit                  | Tidak                 | 10 menit           |
| E-PARSE-01 | Log CRITICAL + kirim sample HTML ke log | Ya (butuh fix manual) | Nonaktifkan sumber |
| E-PARSE-02 | Sama seperti E-PARSE-01                 | Ya                    | Nonaktifkan sumber |
| E-PARSE-03 | Skip record ini, lanjutkan              | Tidak                 | Tidak              |
| E-PARSE-04 | Circuit breaker OPEN                    | Ya                    | 2 jam              |
| E-DATA-01  | Skip (expected behavior)                | Tidak                 | Tidak              |
| E-SYS-01   | Retry + alert                           | Ya                    | Tidak              |

---

## 5. Architecture — Komponen & Interface

### 5.1 Interface Kontrak untuk Scraper

```go
// app/services/scraper/source.go
type ScrapedJob struct {
    SourceID       string    // ID unik dari sumber (URL atau hash)
    Title          string
    Company        string
    Location       string
    Salary         string
    EmploymentType string
    Description    string
    Skills         []string
    ApplyURL       string
    PostedAt       *time.Time // pointer, bisa nil jika tidak tersedia
}

type ScrapeResult struct {
    Jobs      []ScrapedJob
    NextPage  string // URL halaman berikutnya, "" jika tidak ada
    TotalJobs int    // estimasi total (jika sumber menyediakan)
}

type JobSource interface {
    Name()        string            // "loker_id", "karir_com", dll
    DisplayName() string            // "Loker.id", "Karir.com"
    BaseDelay()   time.Duration     // delay minimum antar request
    Jitter()      time.Duration     // maksimal jitter random
    MaxPerHour()  int               // batas request per jam
    Scrape(ctx context.Context, keyword, location string, page int) (ScrapeResult, error)
    IsHealthy()   bool              // false jika circuit breaker open
}
```

### 5.2 Source Registry & Selector Pattern

```go
// app/services/scraper/registry.go
type SourceRegistry struct {
    sources map[string]JobSource
    mu      sync.RWMutex
}

func NewRegistry() *SourceRegistry {
    r := &SourceRegistry{sources: make(map[string]JobSource)}
    r.Register(&LokerIDSource{})
    r.Register(&KarirComSource{})
    r.Register(&IndeedSource{})
    r.Register(&GlintsSource{})
    // tambah sumber baru di sini saja
    return r
}

func (r *SourceRegistry) GetHealthySources() []JobSource {
    r.mu.RLock()
    defer r.mu.RUnlock()
    var healthy []JobSource
    for _, s := range r.sources {
        if s.IsHealthy() {
            healthy = append(healthy, s)
        }
    }
    return healthy
}
```

### 5.3 Queue Driver

Pilihan untuk Goravel (tanpa dependency eksternal dulu):

**Development:** `sync.Channel` in-memory — cukup untuk dev, hilang saat restart
**Production:** Goravel Queue dengan driver `database` (simpan di MongoDB) — tidak perlu Redis terpisah

```go
// config/queue.go
"connections": map[string]any{
    "mongodb": map[string]any{
        "driver":     "mongodb",
        "database":   "loker_radar",
        "collection": "job_queue",
    },
},
```

> **Keputusan:** Mulai dengan MongoDB queue driver — lebih sederhana, tidak perlu setup Redis tambahan. Migrasi ke Redis hanya jika queue volume > 10.000 jobs/hari.

### 5.4 Inbound Rate Limiter

```go
// app/http/middleware/rate_limit.go
// Batasi: 5 search request per IP per menit (gratis, tidak perlu Redis)
// Gunakan in-memory store dengan sync.Map
type RateLimiter struct {
    store sync.Map
    limit int
    window time.Duration
}
```

---

## 6. Failure Scenarios & Mitigation

### 6.1 Scraper Failures

**S1 — Struktur HTML sumber berubah (paling umum)**

- _Symptom:_ `E-PARSE-01` muncul, 0 job berhasil di-parse dari sumber X
- _Detection:_ Jika success_rate sumber < 10% dalam 1 sesi → flag `selector_broken`
- _Mitigation:_ Nonaktifkan sumber secara otomatis; log sample HTML 500 karakter pertama untuk debugging; sumber lain tetap berjalan normal
- _Recovery:_ Update selector di `source.go` → deploy → aktifkan ulang

**S2 — IP diblokir (CAPTCHA / 403)**

- _Symptom:_ `E-HTTP-403` atau `E-PARSE-04` (response challenge)
- _Detection:_ 3 request berturut-turut gagal
- _Mitigation:_ Circuit breaker OPEN; cooldown 1-2 jam; ganti UA pool; kurangi `MaxPerHour` sebesar 50%
- _Recovery:_ Otomatis setelah cooldown (half-open test)

**S3 — Rate limited (429)**

- _Symptom:_ `E-HTTP-429`
- _Detection:_ Status code 429 atau body mengandung "too many requests"
- _Mitigation:_ Backoff eksponensial: 30 detik → 1 menit → 5 menit → circuit breaker
- _Recovery:_ Otomatis

**S4 — Sumber down (503)**

- _Symptom:_ `E-HTTP-503` atau timeout
- _Detection:_ 3x timeout berturut-turut
- _Mitigation:_ Mark `unavailable`, skip selama 30 menit, lanjutkan sumber lain

**S5 — MongoDB penuh / write gagal**

- _Symptom:_ `E-SYS-01`
- _Detection:_ Error dari driver MongoDB
- _Mitigation:_ TTL index wajib dari hari pertama; alert disk > 80%; jangan biarkan `jobs` tumbuh tanpa batas

**S6 — Scheduler tidak berjalan**

- _Symptom:_ Data stagnan, `last_scraped_at` tidak update
- _Detection:_ Healthcheck endpoint: jika `last_scheduler_run > 2 × interval` → return 503
- _Mitigation:_ Pasang external monitor (UptimeRobot / BetterUptime) yang ping `/health` tiap 5 menit

### 6.2 WebSocket Failures

**S7 — Client disconnect saat scraping berjalan**

- _Mitigation:_ Channel non-blocking; skip broadcast jika tidak ada subscriber; data tetap tersimpan di DB

**S8 — Goroutine leak (koneksi tidak ditutup)**

- _Detection:_ Monitor `runtime.NumGoroutine()` via `/health`
- _Mitigation:_ Context dengan timeout; `defer conn.Close()`; max 200 koneksi per IP

**S9 — Burst data (ratusan job sekaligus)**

- _Mitigation:_ Batch broadcast: kumpulkan hasil 2 detik, kirim sebagai array `[]Job`

### 6.3 Frontend Failures

**S10 — WebSocket tidak tersedia**

- _Mitigation:_ Fallback otomatis ke polling `/api/jobs?since=<ts>` setiap 15 detik; banner "Mode lambat aktif"

**S11 — Query tidak menghasilkan hasil dalam 30 detik**

- _Mitigation:_ Empty state direktif + saran keyword populer di lokasi yang sama

---

## 7. Data Model

### 7.1 `jobs` Collection

```json
{
  "_id": "ObjectId",
  "title": "Staff Akuntan",
  "title_normalized": "staff akuntan",
  "company": "PT Maju Bersama",
  "company_normalized": "pt maju bersama",
  "location": "Surabaya",
  "location_slug": "surabaya",
  "category": "akuntansi",
  "salary_raw": "4-6 Juta",
  "salary_min": 4000000,
  "salary_max": 6000000,
  "skills": ["Excel", "SAP", "Pajak"],
  "employment_type": "full-time",
  "description": "...",
  "apply_url": "https://...",
  "source": "loker_id",
  "source_id": "loker-id-job-12345",
  "content_hash": "sha256:abc123...", // hash dari title+company+location untuk fuzzy dedup
  "posted_at": "2025-01-15T00:00:00Z",
  "scraped_at": "2025-01-15T10:30:00Z",
  "expires_at": "2025-01-22T10:30:00Z",
  "is_active": true
}
```

**Deduplication Strategy (3-tier):**

1. **Exact:** `source_id + source` sudah ada → skip (paling cepat)
2. **Content hash:** `sha256(lowercase(title + company + location))` sudah ada dalam 24 jam → skip
3. **Update stale:** Record ada tapi `scraped_at` > 5 hari → update `scraped_at` dan `expires_at`, jangan duplikat

### 7.2 `sources` Collection

```json
{
  "_id": "ObjectId",
  "name": "loker_id",
  "display_name": "Loker.id",
  "status": "active",
  "scrape_type": "html_goquery",
  "selector_version": 3,
  "last_scraped_at": "...",
  "cooldown_until": null,
  "circuit_state": "closed",
  "success_count_24h": 145,
  "error_count_24h": 2,
  "last_error_class": null,
  "last_error_at": null,
  "max_per_hour": 30,
  "base_delay_ms": 3000,
  "jitter_ms": 1000
}
```

### 7.3 `search_queries` Collection

```json
{
  "_id": "ObjectId",
  "keyword": "akuntan",
  "keyword_normalized": "akuntan",
  "location": "Surabaya",
  "location_slug": "surabaya",
  "channel": "akuntan-surabaya",
  "created_at": "...",
  "last_triggered_at": "...",
  "trigger_count": 12,
  "result_count_last": 8,
  "expires_at": "...",
  "status": "active"
}
```

**Lifecycle:** Query yang `result_count_last = 0` setelah 5 trigger berturut-turut → `status = "paused"`. Admin bisa reaktifkan manual atau auto-reaktif jika ada subscriber WebSocket baru untuk keyword tersebut.

### 7.4 Index Wajib

```javascript
db.jobs.createIndex({ source: 1, source_id: 1 }, { unique: true });
db.jobs.createIndex({ content_hash: 1, scraped_at: -1 });
db.jobs.createIndex({ location_slug: 1, category: 1, scraped_at: -1 });
db.jobs.createIndex({ expires_at: 1 }, { expireAfterSeconds: 0 }); // TTL

db.search_queries.createIndex(
  { keyword_normalized: 1, location_slug: 1 },
  { unique: true },
);
db.search_queries.createIndex({ expires_at: 1 }, { expireAfterSeconds: 0 });

db.sources.createIndex({ name: 1 }, { unique: true });
```

---

## 8. Design System — Color, Typography, UX Laws

### 8.1 Filosofi Desain: "Signal, Not Noise"

Loker Radar adalah alat bantu pengambilan keputusan kerja. Pengguna dalam kondisi mencari kerja — state yang seringkali membawa tekanan emosional. Desain harus:

- Memberi rasa **kontrol dan kejelasan** (bukan hype)
- **Mempercepat scanning** informasi (hierarki kuat)
- Terasa **dapat dipercaya** (bukan startup template)

### 8.2 Triadic Color Scheme

Menggunakan triadic harmony dari roda warna dengan rotasi 120° di HSL, semua anchor di Saturation 58%, Lightness 27% agar harmonis secara perseptual.

```
Roda Warna:
  Anchor Primary → H:214  → Biru Prusia   #1C3F6E
  +120°           → H:334  → (Pink/Magenta — SKIP, tidak profesional untuk job board)
  Penyesuaian    → H:154  → Teal Petrol   #1C6E4A  (geser dari H:334 ke complement yang produktif)
  +120° lagi     → H:34   → Amber Tembaga #6E3D1C
```

**Token Warna Lengkap:**

```css
:root {
  /* === TRIADIC CORE === */
  --color-primary: #1c3f6e; /* Biru Prusia */
  --color-primary-light: #2e5fa3; /* Biru medium */
  --color-primary-muted: #e8eff8; /* Biru sangat pucat */
  --color-primary-dark: #112744; /* Biru sangat gelap (hover destructive) */

  --color-teal: #1c6e4a; /* Teal Petrol */
  --color-teal-light: #2ea46e; /* Teal terang */
  --color-teal-muted: #e6f5ee; /* Teal sangat pucat */

  --color-amber: #6e3d1c; /* Amber Tembaga */
  --color-amber-light: #c4713a; /* Amber medium */
  --color-amber-muted: #fbf0e8; /* Amber sangat pucat */

  /* === NEUTRALS (warm-biased) === */
  --color-ink: #12171f; /* Body text utama */
  --color-ink-secondary: #3d4a5c; /* Secondary text */
  --color-ink-tertiary: #7a8a9e; /* Placeholder, disabled */

  --color-surface: #f7f9fc; /* Background halaman */
  --color-surface-card: #ffffff; /* Background kartu */
  --color-border: #d6deea; /* Border default */
  --color-border-strong: #a8b8cc; /* Border hover/focus */

  /* === SEMANTIC === */
  --color-success: var(--color-teal);
  --color-warning: var(--color-amber-light);
  --color-error: #c0392b;
  --color-info: var(--color-primary-light);

  /* === LIVE STATUS === */
  --color-live-pulse: #2ea46e; /* Dot hijau pulsing — WebSocket aktif */
  --color-live-warn: #c4713a; /* Dot amber — reconnecting */
  --color-live-dead: #c0392b; /* Dot merah — disconnected */
}
```

### 8.3 Color Psychology — Justifikasi

| Warna                       | Psikologi                                                                  | Penggunaan                                                     |
| --------------------------- | -------------------------------------------------------------------------- | -------------------------------------------------------------- |
| **Biru Prusia `#1C3F6E`**   | Kepercayaan, stabilitas, profesionalisme. Warna institusi.                 | Header, brand, CTA utama "Cari", link nav                      |
| **Teal Petrol `#1C6E4A`**   | Pertumbuhan, keseimbangan, optimisme realistis. Lebih hangat dari hijau.   | Badge "Baru", status live, salary highlight, konfirmasi sukses |
| **Amber Tembaga `#6E3D1C`** | Urgency ringan, kehangatan, perhatian. Tidak agresif seperti merah.        | Bookmark tersimpan, "Segera dibutuhkan", aksen dekoratif       |
| **Warm Neutral**            | Humanis, tidak steril. Sedikit warm bias mencegah kesan "cold enterprise". | Body text, border, background                                  |

### 8.4 Typography

```css
/* FONT STACK */
--font-display: "DM Serif Display", Georgia, serif; /* Hero, judul besar */
--font-body: "IBM Plex Sans", system-ui, sans-serif; /* Body, label, UI */
--font-mono: "IBM Plex Mono", monospace; /* Gaji, slug, meta data */

/* TYPE SCALE (modular ratio 1.25) */
--text-xs: 0.75rem; /* 12px — timestamp, meta */
--text-sm: 0.875rem; /* 14px — badge, caption */
--text-base: 1rem; /* 16px — body */
--text-md: 1.125rem; /* 18px — subheading */
--text-lg: 1.25rem; /* 20px — section label */
--text-xl: 1.5rem; /* 24px — card title */
--text-2xl: 1.875rem; /* 30px — section heading */
--text-3xl: 2.25rem; /* 36px — hero subtitle */
--text-4xl: 3rem; /* 48px — hero title */
```

---

## 9. UX Laws & UI Rules

### 9.1 UX Laws yang Diterapkan

**Hick's Law** — _Semakin banyak pilihan, semakin lama waktu keputusan._
Implementasi: Form pencarian hanya 3 field utama (keyword, lokasi, kategori). Filter tambahan (gaji, tipe kerja) disembunyikan di balik "Filter Lanjutan" yang collapsible.

**Fitts's Law** — _Target yang lebih besar dan lebih dekat lebih mudah diklik._
Implementasi: Tombol "Cari" lebar penuh di mobile. Area klik kartu = seluruh kartu. Badge dan tombol bookmark minimal 44×44px touch target.

**Miller's Law** — _Otak manusia memproses ~7±2 item sekaligus._
Implementasi: Maksimal 7 skills per kartu (sisanya "+N lagi"). Load 20 item per halaman/batch.

**Jakob's Law** — _Pengguna mengharapkan situs bekerja seperti yang sudah mereka kenal._
Implementasi: Layout kartu lowongan mengikuti konvensi job board (judul besar atas, perusahaan + lokasi bawah, gaji kanan). Jangan reinvent elemen fundamental.

**Law of Proximity** — _Elemen yang berdekatan dianggap berkaitan._
Implementasi: "Perusahaan + Lokasi + Tipe Kerja" satu baris. "Gaji + Tanggal Post" baris berbeda. Tidak campur metadata yang tidak berkaitan.

**Progressive Disclosure** — _Sembunyikan detail sampai dibutuhkan._
Implementasi: Deskripsi dipotong 2 baris + "Selengkapnya". Detail lengkap di drawer/modal, bukan inline.

**Doherty Threshold** — _Sistem harus merespons < 400ms untuk menjaga flow._
Implementasi: Setelah klik "Cari", skeleton card muncul dalam < 100ms. Data actual menyusul via WebSocket. Tidak ada layar kosong menunggu.

**Tesler's Law (Conservation of Complexity)** — _Kompleksitas tidak bisa dihilangkan, hanya dipindahkan._
Implementasi: Kompleksitas scraping multi-sumber, anti-detection, dedup — semuanya di backend. Frontend terasa sederhana.

**Peak-End Rule** — _Pengguna menilai pengalaman dari puncaknya dan akhirnya._
Implementasi: "Peak" = momen pertama lowongan relevan muncul real-time (micro-animation memuaskan). "End" = jika tidak ada hasil, akhiri dengan saran helpful, bukan layar kosong.

### 9.2 UI Rules Spesifik

**Rule 1 — Satu aksi utama per layar**
Halaman utama: aksi utama adalah "Cari". Tidak ada CTA lain yang bersaing (tidak ada "Daftar", "Premium" yang menonjol di hero).

**Rule 2 — Status sistem selalu visible**
WebSocket status ditampilkan sebagai dot persisten:

- 🟢 `--color-live-pulse` pulsing = live, terhubung
- 🟡 `--color-live-warn` = reconnecting
- 🔴 `--color-live-dead` = disconnected, polling aktif

**Rule 3 — Error harus actionable**
Bukan: _"Terjadi kesalahan."_
Tapi: _"Gagal memuat dari Loker.id. Data dari sumber lain tetap ditampilkan. [Coba lagi]"_

**Rule 4 — Empty state harus direktif**
Bukan layar kosong. Tampilkan: ilustrasi ringan + teks spesifik + 3 saran keyword populer.

**Rule 5 — Konsistensi terminologi (pilih satu, patuhi selamanya)**

- "Lowongan" (bukan "job" / "pekerjaan" / "posisi")
- "Simpan" (bukan "bookmark" / "favorit")
- "Cari" (bukan "Temukan" / "Search")
- "Sumber" (bukan "platform" / "portal")

**Rule 6 — Data freshness harus visible**
Setiap kartu menampilkan "Diperbarui 5 mnt lalu" atau "Diposting kemarin". User harus tahu apakah data masih relevan.

**Rule 7 — Animasi harus fungsional**

- Kartu baru: slide-in dari bawah + highlight amber 1.5 detik
- Badge "N baru" di header saat data masuk
- Tidak ada: parallax, rotating text, efek tanpa informasi

**Rule 8 — Mobile-first interaction targets**
Semua interactive element minimum 44×44px. Input minimum `font-size: 16px` (cegah auto-zoom iOS). Jarak antar tombol minimum 8px.

**Rule 9 — Reduce perceived wait time**

1. 0ms: Skeleton card muncul
2. ~500ms: Hasil pertama via WebSocket
3. Ongoing: Append card dengan animasi
4. End: "Pencarian selesai — N lowongan dari M sumber"

**Rule 10 — Sumber data bisa visible tapi tidak ditonjolkan**
Setiap kartu bisa punya tag kecil "via Loker.id" — bukan promosi, tapi transparansi. Ukuran `--text-xs`, warna `--color-ink-tertiary`.

---

## 10. Component Specifications

### 10.1 Job Card

```
┌─────────────────────────────────────────────────────┐
│ [Logo 40px] Staff Akuntan Senior          [Simpan ♡] │
│             PT Maju Bersama                          │
│             📍 Surabaya · Full-time  💰 4–6 Jt/bln  │
├─────────────────────────────────────────────────────┤
│ Excel  SAP  Pajak  +2 lagi    [BARU]  via Loker.id  │
│                                        2 jam lalu   │
├─────────────────────────────────────────────────────┤
│ Bertanggung jawab atas laporan keuangan harian...   │
│ [Selengkapnya ↓]                  [Lamar Sekarang →]│
└─────────────────────────────────────────────────────┘
```

### 10.2 Search Form

```
┌──────────────────────────────────────────────────────────────────────┐
│ 🔍 [Kata kunci, mis. Akuntan, Programmer...]                          │
│ 📍 [Kota atau Provinsi ▼]    📁 [Kategori ▼]    [  Cari Lowongan  ] │
│ [⚙ Filter Lanjutan ▼]                                                 │
│   └─ Gaji minimum: [    ]  Tipe: [ ] Full-time [ ] Part-time [ ] WFH │
└──────────────────────────────────────────────────────────────────────┘
```

### 10.3 Status Bar (Persistent)

```
● Live  •  Mengambil dari 5 sumber  •  47 lowongan  •  Diperbarui 2 mnt lalu
```

Font: `--font-mono`, `--text-xs`, `--color-ink-secondary`. Dot: animasi `pulse` 2s infinite.

### 10.4 Empty State

```
     [ Ilustrasi: kaca pembesar + dokumen kosong, line-art, --color-primary-muted ]

     Belum ada lowongan "Data Analyst" di Makassar.
     Sistem akan terus mencari secara otomatis setiap 5 menit.

     Coba juga:  [Data Engineer]  [Business Analyst]  [Statistisi]
```

---

## 11. Implementation Phases

### Phase 0 — Foundation (Week 1)

- [ ] Setup Goravel project structure
- [ ] Konfigurasi MongoDB connection + migration script (index)
- [ ] `.env` template + `.gitignore` yang benar (wajib: `*.env` dan file secret)
- [ ] Definisikan `JobSource` interface + `ScrapedJob` struct
- [ ] Setup logging structured (JSON, level: debug/info/warn/error)
- [ ] Buat skeleton `SourceRegistry`

### Phase 1 — Scraper Engine (Week 1-2)

- [x] Implementasi `LokerIDSource` (Tipe B — HTML goquery) sebagai sumber pertama
- [x] Anti-detection: UA pool, header realistis, timing jitter
- [x] Circuit breaker per source
- [x] Response validator (`isValidJobPage`)
- [x] `ScraperService` yang iterasi sumber dari registry
- [x] Queue worker (MongoDB driver)
- [x] Scheduler command `scrape_jobs` + `kernel.go`

### Phase 2 — Data Layer (Week 2)

- [x] Model `Job` + `Source` + `SearchQuery`
- [x] `JobRepository` dengan 3-tier deduplication
- [x] Content hash generator (sha256)
- [x] Source status updater (circuit state, error count)
- [x] Unit test: dedup logic, hash collision test

### Phase 3 — Tambah Sumber (Week 2-3)

- [x] `KarirComSource` (Tipe B)
- [x] `IndeedSource` (Tipe C — JSON embedded)
- [x] `GlintsSource` (Tipe C — `__NEXT_DATA__`)
- [x] uTLS integration untuk sumber yang lebih ketat
- [x] Per-source config di `sources` collection (max_per_hour, delay configurable)

### Phase 4 — WebSocket Layer (Week 3)

- [x] WebSocket handler + channel naming (`{keyword-slug}-{location-slug}`)
- [x] Broadcast service (batch mode, 2 detik window)
- [x] Connection pool + goroutine safety (`sync.Map`)
- [x] Fallback polling endpoint `/api/jobs?keyword=&location=&since=<ts>`
- [x] Client reconnect logic (exponential backoff)

### Phase 5 — Frontend (Week 3-4)

- [x] CSS custom properties dari design system Section 8
- [x] Font import: DM Serif Display + IBM Plex Sans/Mono via Google Fonts
- [x] Search form + Filter Lanjutan (collapsible)
- [x] Job card component
- [x] Status bar (WebSocket status dot)
- [x] Skeleton loading states
- [x] Empty state + error state
- [x] WebSocket client dengan fallback polling

### Phase 6 — Polish & Testing (Week 4)

- [ ] Accessibility audit (WCAG 2.1 AA)
- [ ] Responsiveness: 320px → 1440px
- [ ] Cross-browser: Chrome, Firefox, Safari
- [ ] Load test WebSocket (100 koneksi serentak)
- [ ] Anti-detection test: jalankan scraper, cek apakah terblokir dalam 1 jam
- [ ] Error scenario test (source down, parse fail, DB write fail)

### Phase 7 — Deployment (Week 4)

- [ ] Dockerfile (multi-stage build, Go binary kecil)
- [ ] `docker-compose.yml` (app + MongoDB)
- [ ] Nginx config: SSL/TLS + WSS upgrade + gzip
- [ ] DNS setup `loker.elips.site`
- [ ] `/health` endpoint (return scheduler status + source status)
- [ ] External monitor: UptimeRobot ping `/health` tiap 5 menit

---

## 12. Environment & Configuration

### 12.1 `.env` Template

```env
# App
APP_NAME=LokerRadar
APP_ENV=production
APP_PORT=3000
APP_KEY=                          # openssl rand -base64 32

# Database
MONGODB_URI=mongodb://localhost:27017
MONGODB_DATABASE=loker_radar

# Queue (MongoDB driver, tidak perlu Redis)
QUEUE_CONNECTION=mongodb

# Scraper Global
SCRAPER_INTERVAL_MINUTES=5
SCRAPER_REQUEST_TIMEOUT_SECONDS=10
SCRAPER_MAX_RETRIES=3
SCRAPER_CIRCUIT_BREAKER_THRESHOLD=3
SCRAPER_CIRCUIT_COOLDOWN_MINUTES=30

# Per-source override (opsional, bisa juga dari DB)
LOKER_ID_MAX_PER_HOUR=30
KARIR_COM_MAX_PER_HOUR=25
INDEED_MAX_PER_HOUR=20
GLINTS_MAX_PER_HOUR=18
LINKEDIN_MAX_PER_HOUR=10

# TTL
JOB_TTL_DAYS=7
SEARCH_QUERY_TTL_DAYS=30

# Monitoring
HEALTH_CHECK_SECRET=              # optional, untuk proteksi endpoint /health
```

### 12.2 Directory Structure

```
loker-radar/
├── app/
│   ├── http/
│   │   ├── controllers/
│   │   │   └── job_controller.go
│   │   └── middleware/
│   │       └── rate_limit.go
│   ├── models/
│   │   ├── job.go
│   │   ├── search_query.go
│   │   └── source.go
│   ├── repositories/
│   │   └── job_repository.go
│   ├── services/
│   │   ├── scraper/
│   │   │   ├── source.go              ← interface + ScrapedJob struct
│   │   │   ├── registry.go            ← SourceRegistry
│   │   │   ├── circuit_breaker.go     ← CircuitBreaker
│   │   │   ├── anti_detection.go      ← UA pool, header builder, timing
│   │   │   ├── response_validator.go  ← isValidJobPage
│   │   │   ├── loker_id.go
│   │   │   ├── karir_com.go
│   │   │   ├── indeed.go
│   │   │   ├── glints.go
│   │   │   └── kalibrr.go
│   │   ├── scraper_service.go
│   │   └── broadcast_service.go
│   ├── console/
│   │   ├── commands/
│   │   │   ├── scrape_jobs.go
│   │   │   └── cleanup_expired.go     ← cleanup TTL manual jika perlu
│   │   └── kernel.go
│   └── providers/
│       └── database_service_provider.go
├── resources/
│   └── views/
│       ├── welcome.html
│       └── partials/
│           ├── job_card.html
│           └── search_form.html
├── routes/
│   └── web.go
├── config/
│   └── database.go
├── migrations/
│   └── 001_create_indexes.go
├── .env.example
├── .gitignore                         ← wajib: *.env, /vendor, binary
├── Dockerfile
├── docker-compose.yml
└── plan.md
```

---

## 13. Technical Risks Summary

| Risiko                                      | Likelihood        | Impact | Mitigasi                                                  |
| ------------------------------------------- | ----------------- | ------ | --------------------------------------------------------- |
| Struktur HTML sumber berubah                | **Sangat Tinggi** | Tinggi | Response validator + alert E-PARSE-01 + nonaktif otomatis |
| IP diblokir (CAPTCHA)                       | **Tinggi**        | Tinggi | 9-layer anti-detection + circuit breaker + cooldown       |
| Platform pakai JS rendering (SPA tanpa SSR) | Sedang            | Tinggi | Target sumber yang SSR/Next.js dulu; hindari pure SPA     |
| Rate limited (429)                          | **Tinggi**        | Sedang | Per-source delay + jitter + circuit breaker               |
| MongoDB disk penuh                          | Sedang            | Tinggi | TTL index + disk alert 80%                                |
| WebSocket goroutine leak                    | Sedang            | Sedang | Context timeout + connection pool                         |
| Data lowongan usang                         | **Tinggi**        | Sedang | TTL 7 hari + is_active flag                               |
| Scheduler tidak jalan                       | Rendah            | Tinggi | `/health` endpoint + external monitor                     |
| Browser tidak support WS                    | Rendah            | Rendah | Fallback polling otomatis                                 |

---

## 14. Definition of Done

Sebuah fitur dianggap selesai jika:

- [ ] `go build ./...` berhasil tanpa error
- [ ] Unit test untuk logic kritis (dedup, hash, circuit breaker, response validator)
- [ ] Error cases di-handle sesuai Error Taxonomy (Section 4)
- [ ] Tidak ada hardcode value — semua dari `.env`
- [ ] UI ditest di Chrome + Firefox, 375px (mobile) + 1280px (desktop)
- [ ] Tidak ada console error di browser
- [ ] Aksesibilitas: semua interactive element punya visible focus ring + label
- [ ] Anti-detection test: sumber yang ditarget tidak memblokir dalam 60 menit pertama scraping

---

_Dokumen ini adalah living document. Update setiap kali ada keputusan arsitektur baru, perubahan selector sumber, atau penambahan sumber baru._

_Versi: 2.0 (Pure Scraping Edition) | Review berikutnya: setelah Phase 3 selesai_
