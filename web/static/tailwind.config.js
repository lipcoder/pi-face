/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./index.html",
    "./app.js",
    "../**/*.html",
    "../**/*.gohtml",
    "../**/*.tmpl",
    "../**/*.js",
    "../**/*.go",
  ],
  safelist: [
    { pattern: /.*/ }, // 兜底：生成所有匹配到的（会变大，不建议长期用）
  ],
}

