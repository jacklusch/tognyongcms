(function () {
  var toggle = document.querySelector('.nav-toggle');
  var links = document.querySelector('.nav-links');
  if (toggle && links) {
    toggle.addEventListener('click', function () {
      links.classList.toggle('open');
    });
  }
  // 触屏：点击有下拉的父项切换 open（桌面 hover 由 CSS 处理）
  var parents = document.querySelectorAll('.has-children');
  Array.prototype.forEach.call(parents, function (el) {
    var a = el.querySelector(':scope > a');
    if (a) {
      a.addEventListener('click', function (e) {
        if (window.matchMedia && window.matchMedia('(max-width: 900px)').matches) {
          e.preventDefault();
          el.classList.toggle('open');
        }
      });
    }
  });
  // SEO/性能：正文内嵌视频 iframe 懒加载（覆盖已存内容）
  var frames = document.querySelectorAll('.entry-content iframe');
  Array.prototype.forEach.call(frames, function (f) {
    if (!f.hasAttribute('loading')) f.setAttribute('loading', 'lazy');
  });
  // 右下角微信悬浮：点按图标在左侧弹出/收起二维码
  // 注意：悬浮层在 body 模板末尾之后渲染，须等 DOMContentLoaded 后再绑定
  document.addEventListener('DOMContentLoaded', function () {
    var wxBtn = document.querySelector('.float-wechat');
    var wxPop = document.querySelector('.float-wechat-pop');
    if (wxBtn && wxPop) {
      wxBtn.addEventListener('click', function (e) {
        e.preventDefault();
        wxPop.classList.toggle('open');
      });
      document.addEventListener('click', function (e) {
        if (wxPop.classList.contains('open') && !wxPop.contains(e.target) && !wxBtn.contains(e.target)) {
          wxPop.classList.remove('open');
        }
      });
    }
  });
  // 电话/邮件：移动端交给系统拨号与邮件应用；桌面端无对应应用时复制并提示
  document.addEventListener('DOMContentLoaded', function () {
    var ua = navigator.userAgent || '';
    if (/Android|iPhone|iPad|iPod|Opera Mini|IEMobile|Windows Phone|Mobile/i.test(ua)) return;
    var toastEl = null;
    var toastTimer = null;
    function legacyCopy(text) {
      try {
        var ta = document.createElement('textarea');
        ta.value = text;
        ta.setAttribute('readonly', '');
        ta.style.cssText = 'position:fixed;top:0;left:-9999px;opacity:0';
        document.body.appendChild(ta);
        ta.select();
        document.execCommand('copy');
        document.body.removeChild(ta);
      } catch (err) {}
    }
    function copyText(text) {
      try {
        if (navigator.clipboard && navigator.clipboard.writeText) {
          navigator.clipboard.writeText(text)['catch'](function () { legacyCopy(text); });
          return;
        }
      } catch (err) {}
      legacyCopy(text);
    }
    function showToast(html) {
      if (!toastEl) {
        toastEl = document.createElement('div');
        toastEl.className = 'float-toast';
        toastEl.setAttribute('role', 'status');
        toastEl.setAttribute('aria-live', 'polite');
        document.body.appendChild(toastEl);
      }
      toastEl.innerHTML = html;
      toastEl.classList.remove('show');
      void toastEl.offsetWidth;
      toastEl.classList.add('show');
      clearTimeout(toastTimer);
      toastTimer = setTimeout(function () { toastEl.classList.remove('show'); }, 6000);
    }
    function bindClick(selector, value, isEmail) {
      var el = document.querySelector(selector);
      if (!el) return;
      el.addEventListener('click', function () {
        if (!document.hasFocus()) return;
        copyText(value);
        setTimeout(function () {
          // 已唤起拨号/邮件应用时页面会失焦，此时不再打扰
          if (!document.hasFocus()) return;
          if (isEmail) {
            showToast('邮箱已复制：<strong>' + value + '</strong><br><span class="float-toast-hint">若未自动打开邮件应用，可粘贴发送，或</span> <a href="https://mail.google.com/mail/?view=cm&amp;fs=1&amp;to=' + encodeURIComponent(value) + '" target="_blank" rel="noopener">用 Gmail 网页写信</a>');
          } else {
            showToast('电话已复制：<strong>' + value + '</strong><br><span class="float-toast-hint">若未自动打开拨号应用，可在手机拨号盘粘贴拨打</span>');
          }
        }, 900);
      });
    }
    bindClick('.float-phone', '+86 18706678260', false);
    bindClick('.float-email', 'sales@foodsequippro.com', true);
  });
})();
