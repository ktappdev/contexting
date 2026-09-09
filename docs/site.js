(() => {
  const root = document.documentElement;
  root.classList.add("js");

  const themeToggle = document.querySelector(".theme-toggle");
  const copyToast = document.querySelector(".copy-toast");
  const year = document.querySelector("#year");

  const setThemeLabel = () => {
    if (!themeToggle) return;
    const isLight = root.dataset.theme === "light";
    themeToggle.setAttribute("aria-label", isLight ? "Switch to dark theme" : "Switch to light theme");
  };

  themeToggle?.addEventListener("click", () => {
    const nextTheme = root.dataset.theme === "light" ? "dark" : "light";
    root.dataset.theme = nextTheme;
    try {
      localStorage.setItem("ctxt-theme", nextTheme);
    } catch (_error) {
      // The theme still changes for this visit when storage is unavailable.
    }
    setThemeLabel();
  });
  setThemeLabel();

  if (year) year.textContent = String(new Date().getFullYear());

  let toastTimer;
  const showToast = (message) => {
    if (!copyToast) return;
    copyToast.textContent = message;
    copyToast.classList.add("is-visible");
    window.clearTimeout(toastTimer);
    toastTimer = window.setTimeout(() => copyToast.classList.remove("is-visible"), 1600);
  };

  document.querySelectorAll("[data-copy]").forEach((button) => {
    button.addEventListener("click", async () => {
      const value = button.dataset.copy?.replaceAll("\\n", "\n");
      if (!value) return;

      try {
        await navigator.clipboard.writeText(value);
        showToast("Copied to clipboard");
      } catch (_error) {
        showToast("Copy unavailable — select the command");
      }
    });
  });

  const revealItems = document.querySelectorAll(".reveal");
  if ("IntersectionObserver" in window) {
    const observer = new IntersectionObserver(
      (entries, currentObserver) => {
        entries.forEach((entry) => {
          if (!entry.isIntersecting) return;
          entry.target.classList.add("is-visible");
          currentObserver.unobserve(entry.target);
        });
      },
      { rootMargin: "0px 0px -8%", threshold: 0.08 },
    );
    revealItems.forEach((item) => observer.observe(item));
  } else {
    revealItems.forEach((item) => item.classList.add("is-visible"));
  }
})();
