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
})();
