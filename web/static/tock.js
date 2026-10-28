function tockAgo(iso) {
  if (!iso) return "never";
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return iso;
  const delta = Math.round((Date.now() - then) / 1000);
  const abs = Math.abs(delta);
  const future = delta < 0;
  let text;
  if (abs < 45) text = "just now";
  else if (abs < 90) text = "a minute";
  else if (abs < 3600) text = Math.round(abs / 60) + " minutes";
  else if (abs < 5400) text = "an hour";
  else if (abs < 86400) text = Math.round(abs / 3600) + " hours";
  else if (abs < 129600) text = "a day";
  else text = Math.round(abs / 86400) + " days";
  if (text === "just now") return text;
  return future ? "in " + text : text + " ago";
}

function tockCopy(value) {
  if (navigator.clipboard && window.isSecureContext) {
    return navigator.clipboard.writeText(value);
  }
  const area = document.createElement("textarea");
  area.value = value;
  area.setAttribute("readonly", "");
  area.style.position = "fixed";
  area.style.left = "-9999px";
  document.body.appendChild(area);
  area.select();
  const ok = document.execCommand("copy");
  document.body.removeChild(area);
  return ok ? Promise.resolve() : Promise.reject(new Error("copy failed"));
}

document.addEventListener("alpine:init", () => {
  Alpine.data("agoClock", () => ({
    label: "",
    init() {
      const tick = () => { this.label = tockAgo(this.$el.dataset.iso || ""); };
      tick();
      this._timer = setInterval(tick, 30000);
    },
    destroy() { clearInterval(this._timer); }
  }));
  Alpine.data("copyButton", () => ({
    copied: false,
    copy() {
      tockCopy(this.$el.dataset.copy || "").then(() => {
        this.copied = true;
        setTimeout(() => { this.copied = false; }, 1500);
      }).catch(() => {});
    }
  }));
});
