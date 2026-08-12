/* ============================================================
   Audio Talk AI — 官网交互脚本（原生 JS，零依赖）
   ============================================================ */
(function () {
  "use strict";

  /* ---------- 导航：滚动样式 + 移动菜单 ---------- */
  const nav = document.querySelector(".nav");
  const menuBtn = document.querySelector(".menu-btn");
  const navLinks = document.querySelector(".nav-links");

  const onScroll = () => {
    if (window.scrollY > 8) nav.classList.add("scrolled");
    else nav.classList.remove("scrolled");
  };
  onScroll();
  window.addEventListener("scroll", onScroll, { passive: true });

  if (menuBtn && navLinks) {
    menuBtn.addEventListener("click", () => {
      navLinks.classList.toggle("open");
      menuBtn.setAttribute("aria-expanded", navLinks.classList.contains("open"));
    });
    // 点击链接后关闭移动菜单
    navLinks.querySelectorAll("a").forEach((a) =>
      a.addEventListener("click", () => navLinks.classList.remove("open"))
    );
  }

  /* ---------- 滚动入场动画 ---------- */
  const reveals = document.querySelectorAll(".reveal");
  if ("IntersectionObserver" in window && reveals.length) {
    const io = new IntersectionObserver(
      (entries) => {
        entries.forEach((e) => {
          if (e.isIntersecting) {
            e.target.classList.add("visible");
            io.unobserve(e.target);
          }
        });
      },
      { threshold: 0.12, rootMargin: "0px 0px -40px 0px" }
    );
    reveals.forEach((el) => io.observe(el));
  } else {
    reveals.forEach((el) => el.classList.add("visible"));
  }

  /* ---------- 功能卡片光标跟随 ---------- */
  document.querySelectorAll(".feature-card").forEach((card) => {
    card.addEventListener("mousemove", (e) => {
      const r = card.getBoundingClientRect();
      card.style.setProperty("--mx", e.clientX - r.left + "px");
      card.style.setProperty("--my", e.clientY - r.top + "px");
    });
  });

  /* ---------- 平滑滚动（兜底，处理无 CSS scroll-behavior） ---------- */
  document.querySelectorAll('a[href^="#"]').forEach((link) => {
    link.addEventListener("click", (e) => {
      const id = link.getAttribute("href");
      if (id.length > 1) {
        const target = document.querySelector(id);
        if (target) {
          e.preventDefault();
          target.scrollIntoView({ behavior: "smooth", block: "start" });
          history.replaceState(null, "", id);
        }
      }
    });
  });

  /* ---------- OS 标签页切换 ---------- */
  const tabs = document.querySelectorAll(".os-tab");
  const panels = document.querySelectorAll(".os-panel");
  tabs.forEach((tab) => {
    tab.addEventListener("click", () => {
      const target = tab.dataset.os;
      tabs.forEach((t) => t.classList.toggle("active", t === tab));
      panels.forEach((p) => p.classList.toggle("active", p.dataset.os === target));
    });
  });

  /* ---------- 复制按钮 ---------- */
  document.querySelectorAll(".copy-btn").forEach((btn) => {
    btn.addEventListener("click", async () => {
      const block = btn.closest(".code-block");
      const code = block ? block.querySelector("code, pre") : null;
      const text = code ? code.innerText.trim() : "";
      if (!text) return;
      try {
        await navigator.clipboard.writeText(text);
      } catch {
        // 兜底
        const ta = document.createElement("textarea");
        ta.value = text;
        document.body.appendChild(ta);
        ta.select();
        try { document.execCommand("copy"); } catch {}
        document.body.removeChild(ta);
      }
      const orig = btn.innerHTML;
      btn.classList.add("copied");
      btn.querySelector(".copy-label").textContent = btn.dataset.ok || "已复制";
      setTimeout(() => {
        btn.classList.remove("copied");
        btn.innerHTML = orig;
      }, 1800);
    });
  });

  /* ---------- 截图灯箱 ---------- */
  const lightbox = document.querySelector(".lightbox");
  const lbImg = document.querySelector(".lightbox-img");
  if (lightbox && lbImg) {
    document.querySelectorAll(".shot").forEach((shot) => {
      const img = shot.querySelector("img");
      shot.addEventListener("click", () => {
        lbImg.src = img.src;
        lbImg.alt = img.alt;
        lightbox.classList.add("open");
        document.body.style.overflow = "hidden";
      });
    });
    const close = () => {
      lightbox.classList.remove("open");
      document.body.style.overflow = "";
    };
    lightbox.addEventListener("click", (e) => {
      if (e.target === lightbox || e.target.closest(".lightbox-close")) close();
    });
    document.addEventListener("keydown", (e) => {
      if (e.key === "Escape" && lightbox.classList.contains("open")) close();
    });
  }
})();

/* ---------- 最新 release：动态填充版本号与下载地址（GitHub 优先，永不过时）----------
   离线 / 限流时回退到 HTML 里写好的当前版本（v0.2.8）。结果缓存 1 小时，避免频繁打 API。 */
(function () {
  var API = "https://api.github.com/repos/SilverKurali/audio-talk-Ai/releases/latest";
  var CACHE_KEY = "ata-release-v1";
  var TTL = 3600000; // 1h

  var verEls = document.querySelectorAll("[data-latest-version]");
  var assetEls = document.querySelectorAll("[data-asset-url]");
  if (!verEls.length && !assetEls.length) return;

  function apply(d) {
    if (!d || !d.tag_name) return;
    verEls.forEach(function (el) { el.textContent = d.tag_name; });
    var map = {};
    (d.assets || []).forEach(function (a) {
      var m = a.name.match(/(linux|darwin|windows)-(amd64|arm64)\./);
      if (m) map[m[1] + "-" + m[2]] = a.browser_download_url;
    });
    assetEls.forEach(function (el) {
      var url = map[el.getAttribute("data-asset-url")];
      if (!url) return;
      if (el.tagName === "A") el.href = url;
      else el.textContent = url;
    });
  }

  var cached = null;
  try { cached = JSON.parse(localStorage.getItem(CACHE_KEY)); } catch (e) {}
  if (cached && cached.t && Date.now() - cached.t < TTL && cached.d) {
    apply(cached.d);
    return;
  }

  fetch(API, { headers: { Accept: "application/vnd.github+json" } })
    .then(function (r) { return r.ok ? r.json() : null; })
    .then(function (d) {
      if (!d || !d.tag_name) return;
      apply(d);
      try { localStorage.setItem(CACHE_KEY, JSON.stringify({ t: Date.now(), d: d })); } catch (e) {}
    })
    .catch(function () { /* 保留静态回退值 */ });
})();
