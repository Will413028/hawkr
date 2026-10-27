package handler

// docsLoginTemplate 登入頁面
const docsLoginTemplate = `<!DOCTYPE html>
<html lang="zh-TW">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>登入 — Hawkr Docs</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: "Noto Sans TC", "PingFang TC", "Microsoft JhengHei", sans-serif;
    background: #f0f2f5;
    display: flex; justify-content: center; align-items: center; min-height: 100vh;
  }
  .login-card {
    background: #fff; border-radius: 8px; box-shadow: 0 2px 8px rgba(0,0,0,0.1);
    padding: 40px; width: 360px;
  }
  .login-card h1 { font-size: 20px; text-align: center; margin-bottom: 8px; color: #24292e; }
  .login-card p { font-size: 13px; text-align: center; color: #586069; margin-bottom: 24px; }
  .login-card label { display: block; font-size: 13px; font-weight: 600; color: #24292e; margin-bottom: 4px; }
  .login-card input[type="text"],
  .login-card input[type="password"] {
    width: 100%; padding: 8px 12px; border: 1px solid #d1d5da;
    border-radius: 6px; font-size: 14px; margin-bottom: 16px; outline: none;
  }
  .login-card input:focus { border-color: #0366d6; box-shadow: 0 0 0 3px rgba(3,102,214,0.15); }
  .login-card button {
    width: 100%; padding: 10px; background: #2ea44f; color: #fff;
    border: none; border-radius: 6px; font-size: 14px; font-weight: 600; cursor: pointer;
  }
  .login-card button:hover { background: #2c974b; }
</style>
</head>
<body>
<div class="login-card">
  <h1>Hawkr Docs</h1>
  <p>內部文件檢視器，請登入後查看</p>
  <form method="POST" action="/docs/login">
    <label for="username">帳號</label>
    <input type="text" id="username" name="username" autocomplete="username" autofocus>
    <label for="password">密碼</label>
    <input type="password" id="password" name="password" autocomplete="current-password">
    <button type="submit">登入</button>
  </form>
</div>
</body>
</html>`

// docsLoginErrorTemplate 登入失敗頁面
const docsLoginErrorTemplate = `<!DOCTYPE html>
<html lang="zh-TW">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>登入 — Hawkr Docs</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: "Noto Sans TC", "PingFang TC", "Microsoft JhengHei", sans-serif;
    background: #f0f2f5;
    display: flex; justify-content: center; align-items: center; min-height: 100vh;
  }
  .login-card {
    background: #fff; border-radius: 8px; box-shadow: 0 2px 8px rgba(0,0,0,0.1);
    padding: 40px; width: 360px;
  }
  .login-card h1 { font-size: 20px; text-align: center; margin-bottom: 8px; color: #24292e; }
  .login-card p { font-size: 13px; text-align: center; color: #586069; margin-bottom: 24px; }
  .login-card label { display: block; font-size: 13px; font-weight: 600; color: #24292e; margin-bottom: 4px; }
  .login-card input[type="text"],
  .login-card input[type="password"] {
    width: 100%; padding: 8px 12px; border: 1px solid #d1d5da;
    border-radius: 6px; font-size: 14px; margin-bottom: 16px; outline: none;
  }
  .login-card input:focus { border-color: #0366d6; box-shadow: 0 0 0 3px rgba(3,102,214,0.15); }
  .login-card button {
    width: 100%; padding: 10px; background: #2ea44f; color: #fff;
    border: none; border-radius: 6px; font-size: 14px; font-weight: 600; cursor: pointer;
  }
  .login-card button:hover { background: #2c974b; }
  .error-msg {
    background: #ffeef0; border: 1px solid #fdaeb7; color: #86181d;
    padding: 8px 12px; border-radius: 6px; font-size: 13px;
    margin-bottom: 16px; text-align: center;
  }
</style>
</head>
<body>
<div class="login-card">
  <h1>Hawkr Docs</h1>
  <p>內部文件檢視器，請登入後查看</p>
  <div class="error-msg">帳號或密碼錯誤</div>
  <form method="POST" action="/docs/login">
    <label for="username">帳號</label>
    <input type="text" id="username" name="username" autocomplete="username" autofocus>
    <label for="password">密碼</label>
    <input type="password" id="password" name="password" autocomplete="current-password">
    <button type="submit">登入</button>
  </form>
</div>
</body>
</html>`

// docsifyIndexHTML Docsify 單頁應用入口
const docsifyIndexHTML = `<!DOCTYPE html>
<html lang="zh-TW">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Hawkr Docs</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/docsify-themeable@0/dist/css/theme-simple.css">
  <style>
    :root {
      --base-font-family: "Noto Sans TC", "PingFang TC", "Microsoft JhengHei", sans-serif;
      --base-font-size: 15px;
      --base-line-height: 1.7;
      --sidebar-width: 280px;
      --theme-color: #0366d6;
      --content-max-width: none;
    }
    /* 內容區域自適應並置中 */
    .markdown-section {
      max-width: 1200px;
      margin: 0 auto;
      padding: 30px 45px 40px;
    }
    .sidebar-nav > ul > li > a { font-weight: 600; }
    /* 漢堡選單按鈕固定在左上角 */
    .sidebar-toggle {
      position: fixed;
      top: 0;
      left: 0;
      z-index: 30;
    }
    /* Mermaid C4 圖文字間距修正 */
    .mermaid svg text {
      letter-spacing: 0.5px;
    }
    .mermaid svg {
      max-width: 100%;
      height: auto;
    }
    .app-nav { position: fixed; right: 20px; top: 10px; z-index: 10; }
    .app-nav a { font-size: 13px; color: #586069; }
  </style>
</head>
<body>
  <nav class="app-nav"><a href="/docs/logout">登出</a></nav>
  <div id="app">載入中...</div>
  <script>
    window.$docsify = {
      name: 'Hawkr Docs',
      basePath: '/docs/',
      homepage: 'index.md',
      loadSidebar: '_sidebar.md',
      subMaxLevel: 3,
      auto2top: true,
      alias: {
        '/.*/_sidebar.md': '/_sidebar.md'
      },
      search: {
        placeholder: '搜尋文件...',
        noData: '找不到結果',
        paths: 'auto',
        maxAge: 300000
      },
      formatUpdated: '{YYYY}-{MM}-{DD}',
      markdown: {
        renderer: {
          code: function(code, lang) {
            if (lang === 'mermaid') {
              return '<div class="mermaid">' + code + '</div>';
            }
            return this.origin.code.apply(this, arguments);
          }
        }
      }
    };
  </script>
  <script src="https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.min.js"></script>
  <script>mermaid.initialize({
    startOnLoad: false,
    theme: 'default',
    c4: {
      diagramMarginX: 50,
      diagramMarginY: 20,
      c4ShapeMargin: 30,
      c4ShapePadding: 20,
      width: 280,
      height: 120,
      wrap: true,
      wrapPadding: 20,
      personFontSize: 14,
      systemFontSize: 14,
      system_dbFontSize: 14,
      system_queueFontSize: 14,
      external_personFontSize: 14,
      external_systemFontSize: 14
    }
  });</script>
  <script src="https://cdn.jsdelivr.net/npm/docsify@4/lib/docsify.min.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/docsify@4/lib/plugins/search.min.js"></script>
  <script>
    // 每次頁面切換後重新渲染 mermaid
    window.$docsify.plugins = (window.$docsify.plugins || []).concat(function(hook) {
      hook.doneEach(function() {
        mermaid.run({ querySelector: '.mermaid' });
      });
    });
  </script>
  <script src="https://cdn.jsdelivr.net/npm/prismjs@1/components/prism-java.min.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/prismjs@1/components/prism-go.min.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/prismjs@1/components/prism-protobuf.min.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/prismjs@1/components/prism-bash.min.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/prismjs@1/components/prism-yaml.min.js"></script>
  <script src="https://cdn.jsdelivr.net/npm/prismjs@1/components/prism-sql.min.js"></script>
</body>
</html>`
