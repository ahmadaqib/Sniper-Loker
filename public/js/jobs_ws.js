class JobsRealtimeClient {
  constructor({ keyword = "", location = "", onJobs = () => {}, onStatus = () => {} } = {}) {
    this.keyword = keyword;
    this.location = location;
    this.onJobs = onJobs;
    this.onStatus = onStatus;
    this.retry = 0;
    this.ws = null;
    this.pollTimer = null;
    this.lastSeen = new Date(0).toISOString();
  }

  connect() {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const params = new URLSearchParams({ keyword: this.keyword, location: this.location });
    this.ws = new WebSocket(`${protocol}//${window.location.host}/ws/jobs?${params.toString()}`);

    this.ws.onopen = () => {
      this.retry = 0;
      this.stopPolling();
      this.onStatus("connected");
    };

    this.ws.onmessage = (event) => {
      const message = JSON.parse(event.data);
      if (message.type === "batch") {
        message.items.forEach((item) => this.consumeJobs(item.jobs || []));
      } else if (message.type === "jobs") {
        this.consumeJobs(message.jobs || []);
      }
    };

    this.ws.onclose = () => this.reconnect();
    this.ws.onerror = () => {
      this.onStatus("error");
      this.ws.close();
    };
  }

  reconnect() {
    this.onStatus("reconnecting");
    this.startPolling();
    const delay = Math.min(30000, 1000 * Math.pow(2, this.retry++));
    window.setTimeout(() => this.connect(), delay);
  }

  startPolling() {
    if (this.pollTimer) return;
    this.pollTimer = window.setInterval(() => this.poll(), 5000);
    this.poll();
  }

  stopPolling() {
    if (!this.pollTimer) return;
    window.clearInterval(this.pollTimer);
    this.pollTimer = null;
  }

  async poll() {
    const params = new URLSearchParams({
      keyword: this.keyword,
      location: this.location,
      since: this.lastSeen,
    });
    const response = await fetch(`/api/jobs?${params.toString()}`);
    if (!response.ok) return;
    const payload = await response.json();
    this.consumeJobs(payload.jobs || []);
  }

  consumeJobs(jobs) {
    if (!jobs.length) return;
    const dates = jobs.map((job) => job.last_seen_at).filter(Boolean).sort();
    if (dates.length) this.lastSeen = dates[dates.length - 1];
    this.onJobs(jobs);
  }

  close() {
    this.stopPolling();
    if (this.ws) this.ws.close();
  }
}

window.JobsRealtimeClient = JobsRealtimeClient;

