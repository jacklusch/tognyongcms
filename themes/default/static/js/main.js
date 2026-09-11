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
})();
